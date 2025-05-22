Distributed Image-processing Pipeline

# Dataset Credit

This project uses the kodak image dataset for testing(24 The Kodak dataset is a collection of 24 uncompressed 768×512 24-bit RGB images widely used for benchmarking image compression and quality enhancement algorithms.)

https://www.kaggle.com/datasets/sherylmehta/kodak-dataset

you can put the zip file in the input dir to do benchmarks using the go cli

```
curl -L -o ~/Downloads/kodak-dataset.zip\
  https://www.kaggle.com/api/v1/datasets/download/sherylmehta/kodak-dataset
```


The naive implementation writes and reads from disk multiple times to split images and reassemble them
Naive Implementation(local bare metal) Single Threaded 30.3 sec
Naive Implementation(local bare metal) Multi Threaded(8 workers) 9.8 sec (~3x speedup)

memory optimized implmentation only reads from disk once and writes to it once everything else is done in memory
optimized Implementation Single Threaded(local bare metal) 29.4 sec
optimized Implementation Multithreaded(local bare metal)(8 Workers) 8.7 secs

super optimized implementation by eliminating wasm-bindgen overhead and manual memory management using raw pointers for zero-copy data transfer between host and WASM. This reduces runtime cost and improves performance in low-level WASI environments. This also eliminates the serialization, deseralization overhead.

super optimized Implementation Single Threaded(local bare metal) 24.73 sec
super optimized Implementation Multi Threaded(local bare metal)(8 workers) 7.72 sec


The tests were all done on M3 macbook air with 16GB RAM