use std::io::{self, Read, Write};

fn main() -> io::Result<()> {
    let (mut stdin, mut stdout) = (io::stdin().lock(), io::stdout().lock());
    let mut header = [0u8; 4];

    while stdin.read_exact(&mut header).is_ok() {
        let mut doc = vec![0u8; u32::from_be_bytes(header) as usize];
        stdin.read_exact(&mut doc)?;

        let out = html2text::from_read(doc.as_slice(), 65)
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e.to_string()))?;
        stdout.write_all(&(out.len() as u32).to_be_bytes())?;
        stdout.write_all(out.as_bytes())?;
        stdout.flush()?;
    }

    Ok(())
}
