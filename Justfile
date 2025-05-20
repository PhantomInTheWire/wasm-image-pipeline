default: build

build:
  cargo build --manifest-path modules/filter/Cargo.toml --target wasm32-wasi

run-cli:
  go run ./apps/cli

test-pipeline:
  ./scripts/test_pipeline.sh

