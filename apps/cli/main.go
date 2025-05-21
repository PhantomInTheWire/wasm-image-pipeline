package main

import (
    "fmt"
    "image"
    "image/color"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "sync"

    "github.com/disintegration/imaging"
)

const (
    tileSize      = 256
    sharedDir     = "../../shared"
    tilesDir      = sharedDir + "/tiles"
    outputDir     = sharedDir + "/output"
    wasmFilter    = sharedDir + "/filter.wasm"
    inputFilename = sharedDir + "/input/input.png"
    finalImage    = sharedDir + "/output/final.png"
)

func checkErr(err error) {
    if err != nil {
        log.Fatal(err)
    }
}

func runWasmFilter(inPath, outPath string) {  
	cmd := exec.Command("wasmedge", wasmFilter, "process_stdin")

	// Open input tile
	inFile, err := os.Open(inPath)
	if err != nil {
		log.Printf("Failed to open input file %s: %v\n", inPath, err)
		return
	}
	defer inFile.Close()

	// Create output tile
	outFile, err := os.Create(outPath)
	if err != nil {
		log.Printf("Failed to create output file %s: %v\n", outPath, err)
		return
	}
	defer outFile.Close()

	// Attach streams
	cmd.Stdin = inFile
	cmd.Stdout = outFile

	// Run the filter
	if err := cmd.Run(); err != nil {
		log.Printf("ERROR processing %s → %s: %v\n", inPath, outPath, err)
	}
}

func main() {
    // 1. Prepare dirs
    for _, dir := range []string{tilesDir, outputDir} {
        if err := os.RemoveAll(dir); err != nil {
            log.Fatal(err)
        }
        checkErr(os.MkdirAll(dir, 0o755))
    }

    // 2. Load input image
    srcImg, err := imaging.Open(inputFilename)
    checkErr(err)
    bounds := srcImg.Bounds()
    cols := (bounds.Dx() + tileSize - 1) / tileSize
    rows := (bounds.Dy() + tileSize - 1) / tileSize

    // 3. Split into tiles and save
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

    // 4. Process tiles in parallel via WASM filter
    files, err := filepath.Glob(tilesDir + "/*.png")
    checkErr(err)

    fmt.Printf("Processing %d tiles with WASM filter\n", len(files))
    var wg sync.WaitGroup
    for _, inPath := range files {
        wg.Add(1)
        go func(inPath string) {  
			defer wg.Done()  
			outPath := filepath.Join(outputDir, filepath.Base(inPath))  
			runWasmFilter(inPath, outPath)  
		}(inPath)
	}
    wg.Wait()

    // 5. Stitch processed tiles back together
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

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
