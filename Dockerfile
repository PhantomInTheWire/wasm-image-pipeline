# Use the official WasmEdge base image
FROM wasmedge/runwasi:latest
# Copy your compiled filter.wasm into /filters
COPY shared/filter.wasm /filters/gray.wasm

# Expose a simple ENTRYPOINT that
#  • calls WasmEdge on /filters/gray.wasm
#  • passes through any args (in.png out.png)
ENTRYPOINT ["wasmedge", "/filters/gray.wasm", "--"]
