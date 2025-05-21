package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/second-state/WasmEdge-go/wasmedge"
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
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
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

// init loads WasmEdge plugins once
func init() {
	wasmedge.SetLogErrorLevel()
	wasmedge.LoadPluginDefaultPaths()
}

func newVM(wasmPath string) *wasmedge.VM {
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	vm := wasmedge.NewVMWithConfig(conf)
	checkErr(vm.LoadWasmFile(wasmPath))
	checkErr(vm.Validate())
	checkErr(vm.Instantiate())
	return vm
}

func runTile(vm *wasmedge.VM, tile image.Image) (image.Image, error) {
	var inBuf bytes.Buffer
	if err := png.Encode(&inBuf, tile); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	inBytes := inBuf.Bytes()
	inLen := int32(len(inBytes))

	allocRes, err := vm.Execute("alloc", inLen)
	if err != nil {
		return nil, fmt.Errorf("alloc input: %w", err)
	}
	inPtr := allocRes[0].(int32)

	mod := vm.GetActiveModule()
	mem := mod.FindMemory("memory")
	inData, err := mem.GetData(uint(inPtr), uint(inLen))
	if err != nil {
		return nil, fmt.Errorf("mem input: %w", err)
	}
	copy(inData, inBytes)

	outParamsRes, err := vm.Execute("alloc", int32(8))
	if err != nil {
		return nil, fmt.Errorf("alloc params: %w", err)
	}
	outPtrParams := outParamsRes[0].(int32)

	lenRes, err := vm.Execute("grayscale", inPtr, inLen, outPtrParams)
	if err != nil {
		return nil, fmt.Errorf("grayscale: %w", err)
	}
	resultLen := lenRes[0].(int32)
	if resultLen == 0 {
		return nil, fmt.Errorf("zero length output")
	}

	paramBytes, err := mem.GetData(uint(outPtrParams), 8)
	if err != nil {
		return nil, fmt.Errorf("mem params: %w", err)
	}
	outPtr := int32(paramBytes[0]) | int32(paramBytes[1])<<8 | int32(paramBytes[2])<<16 | int32(paramBytes[3])<<24
	outLen := int32(paramBytes[4]) | int32(paramBytes[5])<<8 | int32(paramBytes[6])<<16 | int32(paramBytes[7])<<24

	outData, err := mem.GetData(uint(outPtr), uint(outLen))
	if err != nil {
		return nil, fmt.Errorf("mem output: %w", err)
	}
	outBytes := make([]byte, outLen)
	copy(outBytes, outData)

	img, err := png.Decode(bytes.NewReader(outBytes))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	vm.Execute("dealloc", inPtr, inLen)
	vm.Execute("dealloc", outPtr, outLen)
	vm.Execute("dealloc", outPtrParams, int32(8))

	return img, nil
}

func main() {
	checkErr(os.RemoveAll(outputDir))
	checkErr(os.MkdirAll(outputDir, 0o755))

	var inputs []string
	checkErr(filepath.Walk(inputDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() && filepath.Ext(p) == ".png" {
			inputs = append(inputs, p)
		}
		return nil
	}))
	if len(inputs) == 0 {
		fmt.Printf("No PNGs in %s\n", inputDir)
		return
	}

	// Create VM pool once
	vms := make([]*wasmedge.VM, maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		vms[i] = newVM(wasmFilter)
	}
	defer func() {
		for _, vm := range vms {
			vm.Release()
		}
	}()

	fmt.Printf("Found %d images, processing with %d workers\n", len(inputs), maxWorkers)

	for _, file := range inputs {
		processImage(file, vms)
	}

	fmt.Println("Done")
}

func processImage(file string, vms []*wasmedge.VM) {
	fmt.Printf("→ %s\n", file)
	src, err := imaging.Open(file)
	checkErr(err)

	b := src.Bounds()
	cols := (b.Dx() + tileSize - 1) / tileSize
	rows := (b.Dy() + tileSize - 1) / tileSize

	type task struct {
		x, y int
		tile image.Image
	}
	type result struct {
		x, y int
		img  image.Image
		err  error
	}

	tasks := make(chan task)
	results := make(chan result)

	// launch workers
	for i, vm := range vms {
		go func(vm *wasmedge.VM) {
			for t := range tasks {
				img, err := runTile(vm, t.tile)
				results <- result{t.x, t.y, img, err}
			}
		}(vm)
		_ = i
	}

	// dispatch
	go func() {
		for y := 0; y < rows; y++ {
			for x := 0; x < cols; x++ {
				t := imaging.Crop(src, image.Rect(x*tileSize, y*tileSize,
					x*tileSize+tileSize, y*tileSize+tileSize).Intersect(b))
				tasks <- task{x, y, t}
			}
		}
		close(tasks)
	}()

	tiles := make([][]image.Image, rows)
	for i := range tiles {
		tiles[i] = make([]image.Image, cols)
	}

	// collect
	for i := 0; i < rows*cols; i++ {
		res := <-results
		if res.err != nil {
			log.Fatalf("tile %d,%d error: %v", res.x, res.y, res.err)
		}
		tiles[res.y][res.x] = res.img
	}

	// stitch
	final := imaging.New(b.Dx(), b.Dy(), color.NRGBA{0, 0, 0, 0})
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			final = imaging.Paste(final, tiles[y][x], image.Pt(x*tileSize, y*tileSize))
		}
	}

	outPath := filepath.Join(outputDir,
		filepath.Base(file[:len(file)-len(filepath.Ext(file))])+"_final.png",
	)
	checkErr(imaging.Save(final, outPath))
	fmt.Printf("Saved %s\n", outPath)
}
