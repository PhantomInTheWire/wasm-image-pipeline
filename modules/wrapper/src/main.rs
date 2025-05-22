use std::{
    process::{Command, Stdio},
    fs::File,
    io::{self, Read},
};

fn main() -> io::Result<()> {
    let mut args = std::env::args().skip(1);
    let wasm_path   = args.next().expect("Missing wasm module path");
    let input_path  = args.next().expect("Missing input file path");
    let output_path = args.next().expect("Missing output file path");

    let mut input  = File::open(&input_path)?;
    let mut output = File::create(&output_path)?;

    let mut child = Command::new("wasmedge")
        .arg(&wasm_path)
        .arg("process_stdin")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("Failed to launch wasmedge");

    // 1) Take ownership of stdin, write & then drop it to send EOF
    {
        let mut stdin = child.stdin.take().expect("Failed to open stdin");
        io::copy(&mut input, &mut stdin)?;
        // explicitly close stdin so the wasm sees EOF:
        drop(stdin);
    }

    // 2) Read all of stdout into the output file
    {
        let mut stdout = child.stdout.take().expect("Failed to open stdout");
        io::copy(&mut stdout, &mut output)?;
    }

    // 3) Collect any stderr for diagnostics
    let mut errbuf = Vec::new();
    if let Some(mut stderr) = child.stderr.take() {
        stderr.read_to_end(&mut errbuf)?;
    }

    // 4) Wait for exit and print logs if non-zero
    let status = child.wait()?;
    if !status.success() {
        eprintln!("Wasm exited with status: {}", status);
        eprintln!("stderr:\n{}", String::from_utf8_lossy(&errbuf));
        std::process::exit(1);
    }
    if !errbuf.is_empty() {
        eprintln!("Wasm warnings:\n{}", String::from_utf8_lossy(&errbuf));
    }

    Ok(())
}
