import { convert } from 'html-to-text';

let buf = Buffer.alloc(0);

process.stdin.on('data', (chunk) => {
  buf = Buffer.concat([buf, chunk]);

  while (buf.length >= 4 && buf.length >= 4 + buf.readUInt32BE(0)) {
    const n = buf.readUInt32BE(0);
    const out = Buffer.from(convert(buf.subarray(4, 4 + n).toString('utf8'), { wordwrap: 65 }));
    buf = buf.subarray(4 + n);

    const header = Buffer.alloc(4);
    header.writeUInt32BE(out.length);
    process.stdout.write(Buffer.concat([header, out]));
  }
});
