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
	tilesDir      = filepath.Join(sharedDir, "split", "original") // Base path for original split tiles
	outputDir     = filepath.Join(sharedDir, "split", "processed") // Base path for processed split tiles
	inputDir      = filepath.Join(sharedDir, "input") // Directory containing input images
	wasmFilter    = filepath.Join(sharedDir, "filter.wasm")
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
	// Clean and create base directories
	checkErr(os.RemoveAll(filepath.Join(sharedDir, "split")))
	checkErr(os.RemoveAll(filepath.Join(sharedDir, "output")))
	checkErr(os.MkdirAll(filepath.Join(sharedDir, "split", "original"), 0o755))
	checkErr(os.MkdirAll(filepath.Join(sharedDir, "split", "processed"), 0o755))
	checkErr(os.MkdirAll(filepath.Join(sharedDir, "output"), 0o755))

	var inputFiles []string
	checkErr(filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".png" {
			inputFiles = append(inputFiles, path)
		}
		return nil
	}))

	if len(inputFiles) == 0 {
		fmt.Printf("No PNG files found in %s\n", inputDir)
		return
	}

	fmt.Printf("Found %d PNG files in %s. Processing...\n", len(inputFiles), inputDir)

	for _, inputFilename := range inputFiles {
		fmt.Printf("\nProcessing %s...\n", inputFilename)

		// Determine paths for this specific input file
		relPath, err := filepath.Rel(inputDir, inputFilename)
		checkErr(err)
		baseName := filepath.Base(inputFilename)
		baseNameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]

		currentTilesDir := filepath.Join(tilesDir, filepath.Dir(relPath), baseNameWithoutExt)
		currentOutputDir := filepath.Join(outputDir, filepath.Dir(relPath), baseNameWithoutExt)
		currentFinalImage := filepath.Join(sharedDir, "output", baseNameWithoutExt+"_final.png")

		// Clean and create directories for this input file
		checkErr(os.MkdirAll(currentTilesDir, 0o755))
		checkErr(os.MkdirAll(currentOutputDir, 0o755))

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
				// Use currentTilesDir and new naming
				tilePath := fmt.Sprintf("%s/part_%d_%d.png", currentTilesDir, x, y)
				checkErr(imaging.Save(tile, tilePath))
			}
		}

		// Find tiles in the current original split directory
		files, err := filepath.Glob(filepath.Join(currentTilesDir, "*.png"))
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

				// Use currentOutputDir and new naming
				outPath := filepath.Join(currentOutputDir, filepath.Base(inPath))
				runWasmFilter(inPath, outPath)
			}(inPath)
		}
		wg.Wait()

		fmt.Printf("Stitching processed tiles for %s...\n", inputFilename)
		final := imaging.New(bounds.Dx(), bounds.Dy(), color.NRGBA{0, 0, 0, 0})
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				// Use currentOutputDir and new naming
				tilePath := fmt.Sprintf("%s/part_%d_%d.png", currentOutputDir, x, y)
				tile, err := imaging.Open(tilePath)
				checkErr(err)
				final = imaging.Paste(final, tile, image.Pt(x*tileSize, y*tileSize))
			}
		}
		// Save final image to shared/output with new naming
		checkErr(imaging.Save(final, currentFinalImage))
		fmt.Printf("Done processing %s. Final image written to %s\n", inputFilename, currentFinalImage)
	}

	fmt.Println("\nAll input files processed.")
}
