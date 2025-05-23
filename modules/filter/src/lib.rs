use image::{DynamicImage, ImageFormat};
use std::io::{Cursor, Read, Write};

/// WASI/CLI entry point: read PNG from stdin, grayscale, write PNG to stdout.
/// Processes a PNG image from stdin, converts it to grayscale, and writes the result to stdout.
///
/// # Safety
///
/// This function is unsafe because it directly interacts with the system's standard input and output.
#[unsafe(no_mangle)]
pub extern "C" fn process_stdin() {
    // Read raw bytes from stdin
    let mut buffer = Vec::new();
    std::io::stdin()
        .read_to_end(&mut buffer)
        .expect("Failed to read from stdin");

    // Decode image
    let img = match image::load_from_memory(&buffer) {
        Ok(img) => img,
        Err(e) => {
            eprintln!("Failed to load image: {}", e);
            return;
        }
    };

    // Convert to grayscale
    let gray = img.to_luma8();
    let r#dyn = DynamicImage::ImageLuma8(gray);

    // Encode back to PNG
    let mut out_buf = Vec::new();
    if let Err(e) = r#dyn.write_to(&mut Cursor::new(&mut out_buf), ImageFormat::Png) {
        eprintln!("Failed to encode PNG: {}", e);
        return;
    }

    // Write to stdout
    if let Err(e) = std::io::stdout().write_all(&out_buf) {
        eprintln!("Failed to write PNG to stdout: {}", e);
    }
}

/// In-RAM API: take a PNG byte‐slice, return a freshly-allocated pointer+len to a PNG grayscale.
///
/// # Safety
///
/// The `input_ptr` must point to a valid PNG byte slice of length `input_len`.
/// The `out_ptr` must point to a valid memory location with enough space to write two `u32` values.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn grayscale(
    input_ptr: *const u8,
    input_len: usize,
    out_ptr: *mut u32,
) -> u32 {
    // Safety: we trust the host passed a valid pointer
    let input = unsafe { std::slice::from_raw_parts(input_ptr, input_len) };

    // Decode
    let img = match image::load_from_memory(input) {
        Ok(img) => img,
        Err(_) => return 0, // return 0 length on error
    };

    // Grayscale
    let gray = img.to_luma8();
    let r#dyn = DynamicImage::ImageLuma8(gray);

    // Encode to PNG
    let mut buf = Vec::new();
    if r#dyn
        .write_to(&mut Cursor::new(&mut buf), ImageFormat::Png)
        .is_err()
    {
        return 0;
    }

    // Allocate WASM memory for output
    let len = buf.len() as u32;
    let ptr = alloc(len as usize) as u32;

    // Copy data into WASM memory
    unsafe {
        std::ptr::copy_nonoverlapping(buf.as_ptr(), ptr as *mut u8, len as usize);
    }

    // Write back pointer and length to caller’s out_ptr region
    unsafe {
        // out_ptr points to two u32 slots: [ptr, len]
        std::ptr::write(out_ptr, ptr);
        std::ptr::write(out_ptr.add(1), len);
    }

    // Return length (for convenience)
    len
}

/// Allocate `size` bytes in WASM linear memory and return the base pointer.
///
/// # Safety
///
/// This function is unsafe because it directly interacts with WASM linear memory.
#[unsafe(no_mangle)]
pub extern "C" fn alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

/// Free a previously allocated buffer.
///
/// # Safety
///
/// The `ptr` must point to a valid memory address that was previously allocated by `alloc`.
/// The `size` must match the size that was used when `alloc` was called.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn dealloc(ptr: *mut u8, size: usize) {
    unsafe {
        let _ = Vec::from_raw_parts(ptr, 0, size);
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn grayscale_nomral(input_path: String, output: String) {
    // Read the input image
    let img = match image::open(input_path) {
        Ok(img) => img,
        Err(e) => {
            eprintln!("Failed to load image: {}", e);
            return;
        }
    };

    // Convert to grayscale
    let gray = img.to_luma8();
    let r#dyn = DynamicImage::ImageLuma8(gray);

    // Save the output image
    if let Err(e) = r#dyn.save(output) {
        eprintln!("Failed to save image: {}", e);
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn _start() {
    process_stdin()
}
