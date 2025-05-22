package main

import (
	"fmt"
	"log"
	"os"
	"time"
	"path/filepath"
    "regexp"
    "strings"
	"github.com/PhantomInTheWire/wasm-image-pipeline/pkg/split"
	"github.com/PhantomInTheWire/wasm-image-pipeline/pkg/storage"
	"github.com/PhantomInTheWire/wasm-image-pipeline/pkg/kube"
)

func sanitizeJobName(tile string) string {
	// Extract base without extension
	base := strings.TrimSuffix(filepath.Base(tile), filepath.Ext(tile))

	// Lowercase, replace invalid characters with "-"
	sanitized := regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(strings.ToLower(base), "-")
	sanitized = strings.Trim(sanitized, "-")

	// Append timestamp for uniqueness
	name := fmt.Sprintf("wasm-process-%s-%d", sanitized, time.Now().UnixNano())

	// Ensure total length ≤ 63 characters
	if len(name) > 63 {
		name = name[:63]
	}

	return name
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: controller <image-path>")
		os.Exit(1)
	}

	imagePath := os.Args[1]
	outDir := "./shared/tiles"

	tiles, err := split.Image(imagePath, outDir, 4)
	if err != nil {
		log.Fatalf("error splitting image: %v", err)
	}

	fmt.Println("Tiles created:", tiles)

	minioCfg := storage.MinioConfig{
		Endpoint:  "http://localhost:9000",
		Region:    "us-east-1",
		AccessKey: "minioadmin",  // or from os.Getenv("MINIO_ACCESS_KEY")
		SecretKey: "minioadmin",  // or from os.Getenv("MINIO_SECRET_KEY")
		Bucket:    "tiles-bucket",
		Prefix:    "job1", // you can make this dynamic based on timestamp or image name
		Dir:       outDir,
	}

	if err := storage.UploadTiles(minioCfg); err != nil {
		log.Fatalf("failed to upload tiles to MinIO: %v", err)
	}
	for _, tile := range tiles {
    jobName := sanitizeJobName(tile)
    err := kube.CreateJobForTile(jobName, tile, "default", "http://minio.default.svc:9000/tiles-bucket")
    if err != nil {
        log.Printf("Failed to create job for tile %s: %v", tile, err)
    } else {
        log.Printf("Job created for tile: %s", tile)
    }
}
}
