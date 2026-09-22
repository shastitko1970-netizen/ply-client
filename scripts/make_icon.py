#!/usr/bin/env python3
"""Draw a quiet ink/paper P mark and write PNG + multi-size ICO."""
from __future__ import annotations

import struct
import zlib
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "assets"
INK = (10, 10, 11, 255)
IVORY = (244, 244, 245, 255)


def font_for(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    candidates = [
        "/usr/share/fonts/truetype/liberation/LiberationSerif-Italic.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSerif-Italic.ttf",
        "/usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
        "/usr/share/fonts/truetype/freefont/FreeSerifItalic.ttf",
    ]
    for p in candidates:
        if Path(p).exists():
            return ImageFont.truetype(p, size)
    return ImageFont.load_default()


def draw_mark(px: int) -> Image.Image:
    img = Image.new("RGBA", (px, px), INK)
    draw = ImageDraw.Draw(img)
    f = font_for(int(px * 0.72))
    letter = "P"
    bbox = draw.textbbox((0, 0), letter, font=f)
    tw, th = bbox[2] - bbox[0], bbox[3] - bbox[1]
    x = (px - tw) / 2 - bbox[0]
    y = (px - th) / 2 - bbox[1] - px * 0.03
    # hairline paper fold
    fold = Image.new("RGBA", (px, px), (0, 0, 0, 0))
    fd = ImageDraw.Draw(fold)
    fd.line([(int(px * 0.18), int(px * 0.22)), (int(px * 0.82), int(px * 0.78))], fill=(244, 244, 245, 28), width=max(1, px // 128))
    img = Image.alpha_composite(img, fold.filter(ImageFilter.GaussianBlur(radius=max(0.4, px / 400))))
    ImageDraw.Draw(img).text((x, y), letter, font=f, fill=IVORY)
    return img


def png_bytes(im: Image.Image) -> bytes:
    buf = __import__("io").BytesIO()
    im.save(buf, format="PNG", optimize=True)
    return buf.getvalue()


def write_ico(path: Path, images: list[Image.Image]) -> None:
    # PNG-in-ICO (Vista+)
    entries = []
    blobs = []
    for im in images:
        blob = png_bytes(im)
        w, h = im.size
        entries.append((w if w < 256 else 0, h if h < 256 else 0, blob))
        blobs.append(blob)
    count = len(entries)
    offset = 6 + 16 * count
    out = bytearray()
    out += struct.pack("<HHH", 0, 1, count)
    for w, h, blob in entries:
        out += struct.pack("<BBBBHHII", w, h, 0, 0, 1, 32, len(blob), offset)
        offset += len(blob)
    for blob in blobs:
        out += blob
    path.write_bytes(out)


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    hero = draw_mark(1024)
    hero.save(OUT / "icon.png", "PNG")
    sizes = [16, 24, 32, 48, 64, 128, 256]
    images = [draw_mark(s) for s in sizes]
    write_ico(OUT / "icon.ico", images)
    print("wrote", OUT / "icon.png", OUT / "icon.ico", "bytes", (OUT / "icon.ico").stat().st_size)


if __name__ == "__main__":
    main()
