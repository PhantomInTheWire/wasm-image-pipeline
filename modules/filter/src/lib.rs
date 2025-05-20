use image::{DynamicImage, ImageFormat};
use std::io::{Cursor, Read, Write};

#[cfg(target_arch = "wasm32")]
use wasm_bindgen::prelude::*;

/// Core image-processing function (pure Rust).
/// Expects raw binary image data as input, returns grayscale version.
#[unsafe(no_mangle)]
pub extern "C" fn grayscale_core(input_ptr: *const u8, input_len: usize) {
    // Reconstruct the input slice from the raw pointer and length
    let input = unsafe {
        std::slice::from_raw_parts(input_ptr, input_len)
    };
    
    // Process the image
    let img = image::load_from_memory(input)
        .expect("Failed to load image");
    let gray = img.to_luma8();
    
    // Write the grayscale image to stdout
    let mut out_buf = Vec::new();
    DynamicImage::ImageLuma8(gray)
        .write_to(&mut Cursor::new(&mut out_buf), ImageFormat::Png)
        .expect("Failed to encode PNG");
    
    std::io::stdout().write_all(&out_buf).expect("Failed to write to stdout");
}

/// WASM entry point for CLI usage - reads from stdin and writes to stdout
#[unsafe(no_mangle)]
pub extern "C" fn process_stdin() {
    // Read from stdin
    let mut buffer = Vec::new();
    std::io::stdin().read_to_end(&mut buffer).expect("Failed to read from stdin");
    
    // Process the image
    let img = image::load_from_memory(&buffer)
        .expect("Failed to load image");
    let gray = img.to_luma8();
    
    // Write the grayscale image to stdout
    let mut out_buf = Vec::new();
    DynamicImage::ImageLuma8(gray)
        .write_to(&mut Cursor::new(&mut out_buf), ImageFormat::Png)
        .expect("Failed to encode PNG");
    
    std::io::stdout().write_all(&out_buf).expect("Failed to write to stdout");
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
    
    #[test]
    fn invalid_input_returns_error() {
        let err = grayscale(&[]).expect_err("Empty input should fail");
        assert!(
            err.contains("Failed to load image"),
            "got: {}",
            err
        );
    }
    
    #[test]
    fn roundtrip_grayscale_png() {
        // tiny 1×1 red pixel PNG
        let red_pixel = include_bytes!("../test_assets/red_pixel.png");
        let out = grayscale(red_pixel).expect("Should process valid PNG");
        // PNG magic bytes
        assert_eq!(&out[0..8], b"\x89PNG\r\n\x1a\n");
    }
}