package split

import (
  "fmt"
  "image"
  "image/png"
  "os"
  "path/filepath"
)

// Image splits image at inPath into N×N tiles in outDir,
// returns slice of tile filenames
func Image(inPath string, outDir string, n int) ([]string, error) {
  file, err := os.Open(inPath)
  if err != nil {
    return nil, err
  }
  defer file.Close()
  img, _, err := image.Decode(file)
  if err != nil {
    return nil, err
  }

  bounds := img.Bounds()
  w, h := bounds.Dx(), bounds.Dy()
  tw, th := w/n, h/n

  if err := os.MkdirAll(outDir, 0755); err != nil {
    return nil, err
  }

  tiles := []string{}
  tileID := 0
  for y := 0; y < n; y++ {
    for x := 0; x < n; x++ {
      rect := image.Rect(x*tw, y*th, (x+1)*tw, (y+1)*th).Intersect(bounds)
      sub := img.(interface {
        SubImage(r image.Rectangle) image.Image
      }).SubImage(rect)

      outFile := filepath.Join(outDir, fmt.Sprintf("tile_%d.png", tileID))
      f, err := os.Create(outFile)
      if err != nil {
        return nil, err
      }
      if err := png.Encode(f, sub); err != nil {
        f.Close()
        return nil, err
      }
      f.Close()

      tiles = append(tiles, filepath.Base(outFile))
      tileID++
    }
  }
  return tiles, nil
}
