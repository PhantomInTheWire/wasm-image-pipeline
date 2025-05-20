#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

echo "Checking for required dependencies..."

# Check if cargo is installed
if ! command -v cargo &> /dev/null
then
    echo "Error: cargo is not installed. Please install Rust and Cargo."
    exit 1
fi

# Check if wasmedge is installed
if ! command -v wasmedge &> /dev/null
then
    echo "Error: wasmedge is not installed. Please install WasmEdge."
    exit 1
fi

echo "Dependencies found."

echo "Dependency check finished successfully."