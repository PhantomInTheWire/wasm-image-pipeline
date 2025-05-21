use image::{DynamicImage, ImageBuffer, Rgba, ImageFormat};
use std::io::Cursor;
use filter::{grayscale, alloc, dealloc};

fn create_sample_png() -> Vec<u8> {
    let img: ImageBuffer<Rgba<u8>, _> =
        ImageBuffer::from_fn(2, 2, |x, y| Rgba([(x * 100) as u8, (y * 100) as u8, 150, 255]));
    let mut buf = Vec::new();
    DynamicImage::ImageRgba8(img)
        .write_to(&mut Cursor::new(&mut buf), ImageFormat::Png)
        .unwrap();
    buf
}

#[test]
fn test_alloc_and_dealloc() {
    let size = 64;
    let ptr = alloc(size);
    assert!(!ptr.is_null(), "Allocation returned null pointer");
    unsafe {
        std::ptr::write_bytes(ptr, 0xAB, size);
        dealloc(ptr, size);
    }
}

#[test]
fn test_invalid_input_grayscale() {
    let bad = b"not a png";
    let mut out = [0u32; 2];
    let len = unsafe { grayscale(bad.as_ptr(), bad.len(), out.as_mut_ptr()) };
    assert_eq!(len, 0, "Expected zero length for invalid input");
}

#[test]
fn test_grayscale_round_trip() {
    let img = DynamicImage::new_rgb8(2, 2);
    let mut png = Vec::new();
    img.write_to(&mut Cursor::new(&mut png), ImageFormat::Png).unwrap();

    let mut out = [0u32; 2];
    let len = unsafe { grayscale(png.as_ptr(), png.len(), out.as_mut_ptr()) };
    assert!(len > 0);

    let ptr = out[0] as usize;
    let len_usize = out[1] as usize;
    let slice = unsafe { std::slice::from_raw_parts(ptr as *const u8, len_usize) };

    let gray = image::load_from_memory(slice).unwrap();
    assert_eq!(gray.color(), image::ColorType::L8);

    unsafe { dealloc(out[0] as *mut u8, len_usize) };
}

#[test]
fn test_grayscale_conversion() {
    let input = create_sample_png();
    let mut out = [0u32; 2];
    let len = unsafe { grayscale(input.as_ptr(), input.len(), out.as_mut_ptr()) };
    assert!(len > 0);

    let ptr = out[0] as usize;
    let len_usize = out[1] as usize;
    let slice = unsafe { std::slice::from_raw_parts(ptr as *const u8, len_usize) };    let gray = image::load_from_memory(slice).unwrap();
    assert_eq!(gray.color(), image::ColorType::L8);

    unsafe { dealloc(ptr as *mut u8, len_usize) };
}
