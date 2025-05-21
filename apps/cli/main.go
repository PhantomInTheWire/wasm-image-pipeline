package main

import (
	"bytes"
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

var (
	tileSize   = getEnvInt("TILE_SIZE", 256)
	sharedDir  = getEnv("SHARED_DIR", "../../shared")
	inputDir   = filepath.Join(sharedDir, "input")
	outputDir  = filepath.Join(sharedDir, "output")
	wasmFilter = filepath.Join(sharedDir, "filter.wasm")
	maxWorkers = getEnvInt("MAX_WORKERS", 8)
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

// runWasmFilterInMemory processes an image tile in memory using the WASM filter.
func runWasmFilterInMemory(tile image.Image) (image.Image, error) {
	var inBuf bytes.Buffer
	err := imaging.Encode(&inBuf, tile, imaging.PNG)
	if err != nil {
		return nil, fmt.Errorf("failed to encode tile: %w", err)
	}

	cmd := exec.Command("wasmedge", wasmFilter, "process_stdin")
	cmd.Stdin = &inBuf

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("wasm filter failed: %w", err)
	}

	processed, err := imaging.Decode(&outBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode output tile: %w", err)
	}
	return processed, nil
}

func main() {
	checkErr(os.RemoveAll(outputDir))
	checkErr(os.MkdirAll(outputDir, 0o755))

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

	fmt.Printf("Found %d PNG files. Processing...\n", len(inputFiles))

	for _, inputFile := range inputFiles {
		fmt.Printf("\nProcessing %s...\n", inputFile)
		srcImg, err := imaging.Open(inputFile)
		checkErr(err)

		bounds := srcImg.Bounds()
		cols := (bounds.Dx() + tileSize - 1) / tileSize
		rows := (bounds.Dy() + tileSize - 1) / tileSize

		tiles := make([][]image.Image, rows)
		for i := range tiles {
			tiles[i] = make([]image.Image, cols)
		}

		type job struct {
			x, y  int
			tile  image.Image
		}
		type result struct {
			x, y int
			img  image.Image
			err  error
		}

		jobs := make(chan job)
		results := make(chan result)

		var wg sync.WaitGroup
		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					processed, err := runWasmFilterInMemory(j.tile)
					results <- result{j.x, j.y, processed, err}
				}
			}()
		}

		go func() {
			for y := 0; y < rows; y++ {
				for x := 0; x < cols; x++ {
					x0, y0 := x*tileSize, y*tileSize
					x1 := min(x0+tileSize, bounds.Dx())
					y1 := min(y0+tileSize, bounds.Dy())
					tile := imaging.Crop(srcImg, image.Rect(x0, y0, x1, y1))
					jobs <- job{x, y, tile}
				}
			}
			close(jobs)
		}()

		go func() {
			wg.Wait()
			close(results)
		}()

		for res := range results {
			if res.err != nil {
				log.Fatalf("Tile (%d,%d) failed: %v", res.x, res.y, res.err)
			}
			tiles[res.y][res.x] = res.img
		}

		// Stitch final image
		final := imaging.New(bounds.Dx(), bounds.Dy(), color.NRGBA{0, 0, 0, 0})
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				final = imaging.Paste(final, tiles[y][x], image.Pt(x*tileSize, y*tileSize))
			}
		}

		outName := filepath.Join(outputDir, filepath.Base(inputFile[:len(inputFile)-len(filepath.Ext(inputFile))]+"_final.png"))
		checkErr(imaging.Save(final, outName))
		fmt.Printf("Saved final image: %s\n", outName)
	}
	fmt.Println("\nAll input files processed.")
}
