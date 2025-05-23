package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/disintegration/imaging"
)

// Default grid size
const rows = 4
const cols = 4

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input.png> <output.png>\n", os.Args[0])
		os.Exit(1)
	}
	inPath := os.Args[1]
	outPath := os.Args[2]

	// Load source image
	src, err := imaging.Open(inPath)
	if err != nil {
		log.Fatalf("failed to open input image: %v", err)
	}

	// Compute tile size
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	tw, th := w/cols, h/rows

	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "kfilter-tiles")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Split into tiles
	tilePaths := make([]string, 0, rows*cols)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			x0, y0 := c*tw, r*th
			x1, y1 := x0+tw, y0+th
			tile := imaging.Crop(src, image.Rect(x0, y0, x1, y1))
			fn := filepath.Join(tmpDir, fmt.Sprintf("tile_%d_%d.png", r, c))
			if err := imaging.Save(tile, fn); err != nil {
				log.Fatalf("failed to save tile: %v", err)
			}
			tilePaths = append(tilePaths, fn)
		}
	}

	// Process tiles in parallel
	var wg sync.WaitGroup
	processed := make([]string, len(tilePaths))
	for i, tp := range tilePaths {
		wg.Add(1)
		go func(idx int, inFile string) {
			defer wg.Done()

			outFile := filepath.Join(tmpDir, fmt.Sprintf("proc_%s", filepath.Base(inFile)))
			// Call OCI container (assumes gray-filter:latest reads args)
			cmd := exec.Command("docker", "run", "--rm",
				"-v", fmt.Sprintf("%s:/data", tmpDir),
				"gray-filter:latest", "/data/"+filepath.Base(inFile), "/data/"+filepath.Base(outFile))
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Fatalf("failed to process tile %s: %v, output: %s", inFile, err, string(out))
			}

			processed[idx] = outFile
		}(i, tp)
	}
	wg.Wait()

	// Stitch back
	dst := imaging.New(w, h, image.Transparent)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			pf := processed[idx]
			tileImg, err := imaging.Open(pf)
			if err != nil {
				log.Fatalf("failed to open processed tile: %v", err)
			}
			off := image.Pt(c*tw, r*th)
			draw.Draw(dst, tileImg.Bounds().Add(off), tileImg, image.Point{}, draw.Over)
		}
	}

	// Save final image
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, dst); err != nil {
		log.Fatalf("failed to encode output PNG: %v", err)
	}

	fmt.Printf("Wrote output to %s\n", outPath)
}
