#!/usr/bin/env python3
"""Generate the placeholder Tauri icon set using stdlib only (no PIL).

Run from repo root: `python3 scripts/gen-icon.py`. Produces a valid RGBA PNG
at every size Tauri/macOS bundling expects. Replace these with a real icon
before shipping a release bundle.
"""
import struct
import zlib
from pathlib import Path

OUT_DIR = Path(__file__).resolve().parent.parent / "crates" / "desktop" / "icons"
COLOR = (74, 144, 226, 255)  # RGBA — calm blue
SIZES = [32, 128, 256, 512, 1024]


def png_bytes(width: int, height: int, rgba: tuple[int, int, int, int]) -> bytes:
    raw = bytearray()
    row = bytes(rgba) * width
    for _ in range(height):
        raw.append(0)  # filter byte: None
        raw.extend(row)

    def chunk(kind: bytes, data: bytes) -> bytes:
        return (
            struct.pack(">I", len(data))
            + kind
            + data
            + struct.pack(">I", zlib.crc32(kind + data))
        )

    ihdr = struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)
    idat = zlib.compress(bytes(raw))
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", idat) + chunk(b"IEND", b"")


def main() -> None:
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    for size in SIZES:
        path = OUT_DIR / (f"icon.png" if size == 32 else f"icon-{size}.png")
        path.write_bytes(png_bytes(size, size, COLOR))
        print(f"wrote {path} ({size}x{size})")


if __name__ == "__main__":
    main()
