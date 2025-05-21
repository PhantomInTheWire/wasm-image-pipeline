package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/disintegration/imaging"
)

// Default values (can be overridden via environment variables)
var (
	tileSize      = getEnvInt("TILE_SIZE", 256)
	sharedDir     = getEnv("SHARED_DIR", "../../shared")
	tilesDir      = filepath.Join(sharedDir, "tiles")
	outputDir     = filepath.Join(sharedDir, "output")
	inputFilename = filepath.Join(sharedDir, "input", "input.png")
	wasmFilter    = filepath.Join(sharedDir, "filter.wasm")
	finalImage    = filepath.Join(sharedDir, "output", "final.png")
	maxWorkers    = getEnvInt("MAX_WORKERS", 8)
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func runWasmFilter(inPath, outPath string) {
	cmd := exec.Command("wasmedge", wasmFilter, "process_stdin")

	inFile, err := os.Open(inPath)
	if err != nil {
		log.Printf("Failed to open input file %s: %v\n", inPath, err)
		return
	}
	defer inFile.Close()

	outFile, err := os.Create(outPath)
	if err != nil {
		log.Printf("Failed to create output file %s: %v\n", outPath, err)
		return
	}
	defer outFile.Close()

	cmd.Stdin = inFile
	cmd.Stdout = outFile

	if err := cmd.Run(); err != nil {
		log.Printf("ERROR processing %s → %s: %v\n", inPath, outPath, err)
	}
}

func main() {
	for _, dir := range []string{tilesDir, outputDir} {
		if err := os.RemoveAll(dir); err != nil {
			log.Fatal(err)
		}
		checkErr(os.MkdirAll(dir, 0o755))
	}

	srcImg, err := imaging.Open(inputFilename)
	checkErr(err)
	bounds := srcImg.Bounds()
	cols := (bounds.Dx() + tileSize - 1) / tileSize
	rows := (bounds.Dy() + tileSize - 1) / tileSize

	fmt.Printf("Splitting %dx%d image into %dx%d tiles of %dpx\n",
		bounds.Dx(), bounds.Dy(), cols, rows, tileSize)

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			x0, y0 := x*tileSize, y*tileSize
			x1 := min(x0+tileSize, bounds.Dx())
			y1 := min(y0+tileSize, bounds.Dy())
			tile := imaging.Crop(srcImg, image.Rect(x0, y0, x1, y1))
			tilePath := fmt.Sprintf("%s/tile_%d_%d.png", tilesDir, x, y)
			checkErr(imaging.Save(tile, tilePath))
		}
	}

	files, err := filepath.Glob(filepath.Join(tilesDir, "*.png"))
	checkErr(err)

	fmt.Printf("Processing %d tiles with WASM filter (max concurrency: %d)\n", len(files), maxWorkers)

	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, inPath := range files {
		wg.Add(1)
		go func(inPath string) {
			sem <- struct{}{} // acquire
			defer func() {
				<-sem // release
				wg.Done()
			}()

			outPath := filepath.Join(outputDir, filepath.Base(inPath))
			runWasmFilter(inPath, outPath)
		}(inPath)
	}
	wg.Wait()

	final := imaging.New(bounds.Dx(), bounds.Dy(), color.NRGBA{0, 0, 0, 0})
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			tilePath := fmt.Sprintf("%s/tile_%d_%d.png", outputDir, x, y)
			tile, err := imaging.Open(tilePath)
			checkErr(err)
			final = imaging.Paste(final, tile, image.Pt(x*tileSize, y*tileSize))
		}
	}
	checkErr(imaging.Save(final, finalImage))
	fmt.Printf("Done! Final image written to %s\n", finalImage)

}
