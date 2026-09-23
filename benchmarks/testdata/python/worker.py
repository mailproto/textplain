import struct
import sys

from inscriptis import get_text

stdin, stdout = sys.stdin.buffer, sys.stdout.buffer

while header := stdin.read(4):
    (n,) = struct.unpack(">I", header)
    out = get_text(stdin.read(n).decode()).encode()
    stdout.write(struct.pack(">I", len(out)) + out)
    stdout.flush()
