# 📸 Distributed Image-processing Pipeline

## 🧾 Summary

The project focuses on benchmarking different strategies for distributed image processing using Go and WebAssembly (WASM). The pipeline processes batches of images by splitting each into tiles, applying filters (via WASM modules), and reassembling them.

We compare naive disk-based approaches with increasingly optimized in-memory implementations. Performance gains are achieved by reducing disk I/O and minimizing overhead from WASM interop.

---

## ⚙️ How It Works

Each version of the pipeline follows this general structure:

1. Load an image (or a set of images).
2. Split each image into tiles.
3. Apply a WASM-based filter to each tile.
4. Reassemble the tiles into a final image.

### 🧱 Naive Implementation

* Written in Go.
* Uses the standard image and io packages.
* Splits and saves tiles to disk.
* Loads each tile back into memory for WASM processing.
* Saves the processed tile again, and reassembles from disk.

➡️ Major bottleneck: Excessive disk reads/writes.

### 🧠 Optimized Implementation (In-Memory)

* Still in Go.
* Tiles are created and kept in memory (slices of `image.Image`).
* Processed tiles are passed through pipes directly into WASM and collected via stdout.
* Only one read/write to disk: once for loading, once for final output.

➡️ Benefit: Avoids intermediate disk I/O.

### ⚡ Super Optimized Implementation (Zero-Copy + Raw Pointers)

* Written using Go for orchestration, but uses Rust-compiled WASM modules.
* WASM filter module uses raw pointers and manual memory management for direct buffer access.
* Avoids wasm-bindgen and serialization overhead.
* Tile data is passed directly using WASI stdin/stdout streams in binary form.

➡️ Result: Best performance due to minimal syscall and memory copy overhead.

---

## 📚 Dataset Credit

This project uses the [Kodak image dataset](https://www.kaggle.com/datasets/sherylmehta/kodak-dataset), a set of 24 uncompressed 768×512 RGB images widely used for evaluating compression algorithms.

To benchmark using the Go CLI:

```bash
curl -L -o ~/Downloads/kodak-dataset.zip \
  https://www.kaggle.com/api/v1/datasets/download/sherylmehta/kodak-dataset
```

Unzip the dataset into the `input/` directory.

---

## 🧪 Benchmark Results

| Implementation                   | Mode                       | Time (sec) | Description                                       |
| -------------------------------- | -------------------------- | ---------- | ------------------------------------------------- |
| Naive                            | Single-threaded            | 30.3       | Disk I/O heavy, basic file-based processing       |
| Naive                            | Multi-threaded (8 workers) | 9.8        | Parallelized disk-based processing (\~3× speedup) |
| Optimized (in-memory)            | Single-threaded            | 29.4       | Reduced I/O, all tiles kept in memory             |
| Optimized (in-memory)            | Multi-threaded (8 workers) | 8.7        | Faster parallel in-memory WASM tile filtering     |
| Super Optimized (zero-copy, raw pointers) | Single-threaded            | 24.73      | Raw pointer WASM, no serialization overhead       |
| Super Optimized (zero-copy, raw pointers) | Multi-threaded (8 workers) | 7.72       | Best performance: minimal I/O + zero-copy WASM    |

🧪 All benchmarks were run on an M3 MacBook Air with 16GB RAM.

---