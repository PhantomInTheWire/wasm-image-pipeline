use image::{DynamicImage, ImageFormat};
use std::io::{Cursor, Read, Write};

#[cfg(target_arch = "wasm32")]
use wasm_bindgen::prelude::*;

/// WASM entry point for CLI usage - reads from stdin and writes to stdout
/// This is the main entry point for the WASM module.
/// It reads image data from stdin, processes it, and writes the result to stdout.
/// It is marked as `unsafe` because it interacts with raw pointers and external I/O.
/// The name of this function is not supposed to be mangled, as it is called from foreign code.
#[unsafe(no_mangle)]
pub extern "C" fn process_stdin() {
    // Read from stdin
    let mut buffer = Vec::new();
    std::io::stdin().read_to_end(&mut buffer).expect("Failed to read from stdin");
    
    eprintln!("Read {} bytes from stdin", buffer.len());
    
    // Process the image
    let img = match image::load_from_memory(&buffer) {
        Ok(img) => {
            eprintln!("Successfully loaded image, dimensions: {}x{}", img.width(), img.height());
            img
        },
        Err(e) => {
            eprintln!("Failed to load image: {}", e);
            return;
        }
    };
    
    eprintln!("Converting to grayscale...");
    let gray = img.to_luma8();
    
    
    let mut out_buf = Vec::new();
    match DynamicImage::ImageLuma8(gray).write_to(&mut Cursor::new(&mut out_buf), ImageFormat::Png) {
        Ok(_) => {
            eprintln!("Successfully encoded PNG, output size: {} bytes", out_buf.len());
            if out_buf.len() >= 8 {
                eprintln!("PNG signature check: {:?}", &out_buf[0..8]);
            } else {
                eprintln!("WARNING: Output buffer too small ({} bytes)!", out_buf.len());
                return;
            }
        },
        Err(e) => {
            eprintln!("Failed to encode PNG: {}", e);
            return;
        }
    };
    
    match std::io::stdout().write_all(&out_buf) {
        Ok(_) => {
            eprintln!("Successfully wrote {} bytes to stdout", out_buf.len());
            // Make sure we flush stdout
            if let Err(e) = std::io::stdout().flush() {
                eprintln!("Warning: Failed to flush stdout: {}", e);
            }
        },
        Err(e) => {
            eprintln!("Failed to write to stdout: {}", e);
            panic!("Writing output failed");
        }
    };
}

/// WASM entrypoint: maps Rust errors into JS exceptions.
#[cfg(target_arch = "wasm32")]
#[wasm_bindgen]
pub fn grayscale(input: &[u8]) -> Result<Vec<u8>, JsValue> {
    // Decode with error mapping
    let img = image::load_from_memory(input)
        .map_err(|e| JsValue::from_str(&format!("Failed to load image: {}", e)))?;
    
    // Apply grayscale
    let gray = img.to_luma8();
    let mut buf = Vec::new();
    DynamicImage::ImageLuma8(gray)
        .write_to(&mut Cursor::new(&mut buf), ImageFormat::Png)
        .map_err(|e| JsValue::from_str(&format!("Failed to encode PNG: {}", e)))?;
    
    Ok(buf)
}

/// Native entrypoint to mirror the WASM API shape for desktop tests.
/// Returns `Err(String)` instead of panicking inside the test harness.
#[cfg(not(target_arch = "wasm32"))]
pub fn grayscale(input: &[u8]) -> Result<Vec<u8>, String> {
    let img = match image::load_from_memory(input) {
        Ok(img) => img,
        Err(e) => return Err(format!("Failed to load image: {}", e)),
    };
    
    let gray = img.to_luma8();
    let mut buf = Vec::new();
    
    match DynamicImage::ImageLuma8(gray).write_to(&mut Cursor::new(&mut buf), ImageFormat::Png) {
        Ok(_) => Ok(buf),
        Err(e) => Err(format!("Failed to encode PNG: {}", e)),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // disable ide inspection for this function
    //noinspection ALL
    #[test]
    fn invalid_input_returns_error() {
        let err = grayscale(&[]).expect_err("Empty input should fail");
        assert!(
            err.contains("Failed to load image"),
            "got: {:?}",
            err
        );
    }
    
    #[test]
    fn round_trip_grayscale_png() {
        let red_pixel = include_bytes!("../test_assets/red_pixel.png");
        let out = grayscale(red_pixel).expect("Should process valid PNG");
        // PNG magic bytes for "gray-scaled" red pixel
        assert_eq!(&out[0..8], b"\x89PNG\r\n\x1a\n");
    }
}