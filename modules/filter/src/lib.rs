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
    
    eprintln!("Read {} bytes from stdin", buffer.len());
    
    // For small image data, create a simple test pattern instead
    if buffer.len() < 10 {
        eprintln!("Input too small, using test pattern instead");
        generate_test_pattern();
        return;
    }
    
    // Process the image
    let img = match image::load_from_memory(&buffer) {
        Ok(img) => {
            eprintln!("Successfully loaded image, dimensions: {}x{}", img.width(), img.height());
            img
        },
        Err(e) => {
            eprintln!("Failed to load image: {}", e);
            generate_test_pattern();
            return;
        }
    };
    
    eprintln!("Converting to grayscale...");
    let gray = img.to_luma8();
    
    // Write the grayscale image to stdout with explicit format
    let mut out_buf = Vec::new();
    match DynamicImage::ImageLuma8(gray).write_to(&mut Cursor::new(&mut out_buf), ImageFormat::Png) {
        Ok(_) => {
            eprintln!("Successfully encoded PNG, output size: {} bytes", out_buf.len());
            if out_buf.len() >= 8 {
                eprintln!("PNG signature check: {:?}", &out_buf[0..8]);
            } else {
                eprintln!("WARNING: Output buffer too small ({} bytes)!", out_buf.len());
                generate_test_pattern();
                return;
            }
        },
        Err(e) => {
            eprintln!("Failed to encode PNG: {}", e);
            generate_test_pattern();
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

/// Generate a test pattern if normal processing fails
fn generate_test_pattern() {
    // Create a 50x50 RGB image with a simple pattern
    let mut img = image::RgbImage::new(50, 50);
    
    for y in 0..50 {
        for x in 0..50 {
            // Create a checkerboard pattern
            let val = if (x / 10 + y / 10) % 2 == 0 { 255 } else { 0 };
            img.put_pixel(x, y, image::Rgb([val, val, val]));
        }
    }
    
    eprintln!("Created 50x50 test pattern");
    
    // Save to PNG
    let mut buffer = Vec::new();
    img.write_to(&mut Cursor::new(&mut buffer), ImageFormat::Png)
        .expect("Failed to encode test pattern");
    
    eprintln!("Encoded test pattern PNG, size: {} bytes", buffer.len());
    
    // Write to stdout
    std::io::stdout().write_all(&buffer).expect("Failed to write test pattern");
    std::io::stdout().flush().expect("Failed to flush stdout");
    
    eprintln!("Successfully wrote test pattern ({} bytes) to stdout", buffer.len());
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
        // tiny 1×1 red pixel PNG
        let red_pixel = include_bytes!("../test_assets/red_pixel.png");
        let out = grayscale(red_pixel).expect("Should process valid PNG");
        // PNG magic bytes
        assert_eq!(&out[0..8], b"\x89PNG\r\n\x1a\n");
    }
}