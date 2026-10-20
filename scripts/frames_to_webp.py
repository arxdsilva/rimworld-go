#!/usr/bin/env python3
"""Encode captured PNG frames into a looping animated WebP for the README.

Usage:
  python3 scripts/frames_to_webp.py --frames 'bin/clips/x/f_*.png' --out docs/clips/x.webp
"""
import argparse
import glob
import os
import sys

from PIL import Image


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--frames", required=True, help="glob of input PNG frames")
    p.add_argument("--out", required=True)
    p.add_argument("--width", type=int, default=800)
    p.add_argument("--step", type=int, default=2, help="keep every Nth frame")
    p.add_argument("--fps", type=int, default=30)
    p.add_argument("--quality", type=int, default=70)
    p.add_argument("--start", type=int, default=0)
    p.add_argument("--end", type=int, default=-1)
    p.add_argument("--max-bytes", type=int, default=2_000_000)
    a = p.parse_args()

    files = sorted(glob.glob(a.frames))
    end = None if a.end < 0 else a.end
    files = files[a.start:end][::a.step]
    if not files:
        sys.exit("no frames matched %r" % a.frames)

    frames = []
    for f in files:
        im = Image.open(f).convert("RGB")
        h = round(im.height * a.width / im.width)
        frames.append(im.resize((a.width, h), Image.LANCZOS))

    os.makedirs(os.path.dirname(os.path.abspath(a.out)), exist_ok=True)
    frames[0].save(a.out, save_all=True, append_images=frames[1:], loop=0,
                   duration=round(1000 / a.fps), quality=a.quality, method=6)
    size = os.path.getsize(a.out)
    print("%s: %d frames, %d bytes" % (a.out, len(frames), size))
    if size > a.max_bytes:
        sys.exit("error: %d bytes exceeds max %d" % (size, a.max_bytes))


if __name__ == "__main__":
    main()
