#!/bin/sh
curl -s "$INPUT_URL" | /filter.wasm > /tmp/out.png && \
  curl -X PUT -T /tmp/out.png "$OUTPUT_URL"
