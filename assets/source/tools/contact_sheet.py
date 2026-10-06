#!/usr/bin/env python3
"""Contact sheet of generated world sprites (ART spec §25, §29).

    python3 assets/source/tools/contact_sheet.py [family ...] [-o out.png]

Tintable sprites are shown with a preset tint from palettes/base.yaml so
style drift is visible in context. Dev aid only; the game never reads it.
"""
import argparse
import os
import re
import sys

from PIL import Image, ImageDraw

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
GEN = os.path.join(ROOT, "generated", "world")
META = os.path.join(ROOT, "metadata")

PREVIEW_TINT = {"cloth": "navy", "skin": "tan", "hair": "brown"}


def tints():
    out, group = {}, None
    for line in open(os.path.join(ROOT, "palettes", "base.yaml"), encoding="utf-8"):
        m = re.match(r"^\s{2}(\w+):\s*\{(.*)\}", line)
        if m:
            out[m.group(1)] = dict(re.findall(r'(\w+):\s*"(#[0-9a-fA-F]{6})"', m.group(2)))
    return out


def hexrgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def meta_of(family, name):
    p = os.path.join(META, family, name + ".yaml")
    return open(p, encoding="utf-8").read() if os.path.exists(p) else ""


def apply_tint(img, mask, rgb):
    img = img.convert("RGBA")
    m = mask.convert("L")
    tinted = Image.merge("RGBA", [
        Image.eval(img.getchannel(i), lambda v, c=c: v * c // 255) for i, c in enumerate(rgb)
    ] + [img.getchannel("A")])
    return Image.composite(tinted, img, m)


def tinted(path, family, name, tint_choice=None):
    img = Image.open(path).convert("RGBA")
    md = meta_of(family, name)
    g = re.search(r"\n\s+group: (\w+)", md)
    if g:
        mask_p = os.path.join(os.path.dirname(path), name + "_mask.png")
        if os.path.exists(mask_p):
            t = tints()[g.group(1)]
            key = (tint_choice or {}).get(g.group(1), PREVIEW_TINT[g.group(1)])
            img = apply_tint(img, Image.open(mask_p), hexrgb(t[key]))
    return img


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("families", nargs="*")
    ap.add_argument("-o", "--out", default=os.path.join(ROOT, "generated", "contact_sheet.png"))
    ap.add_argument("--bg", default="#86704f")
    a = ap.parse_args()
    fams = a.families or sorted(os.listdir(GEN))
    items = []
    for fam in fams:
        d = os.path.join(GEN, fam)
        for f in sorted(os.listdir(d)):
            if f.endswith(".png") and not f.endswith("_mask.png"):
                items.append((fam, f[:-4], tinted(os.path.join(d, f), fam, f[:-4])))
    if not items:
        sys.exit("nothing to show")
    cell_w = max(max(i[2].width for i in items), 160) + 16
    cell_h = max(i[2].height for i in items) + 30
    cols = max(1, min(8, 2400 // cell_w))
    rows = (len(items) + cols - 1) // cols
    sheet = Image.new("RGBA", (cols * cell_w, rows * cell_h), hexrgb(a.bg) + (255,))
    dr = ImageDraw.Draw(sheet)
    for i, (fam, name, img) in enumerate(items):
        x, y = (i % cols) * cell_w, (i // cols) * cell_h
        sheet.alpha_composite(img, (x + (cell_w - img.width) // 2, y + 4))
        dr.text((x + 6, y + cell_h - 20), "%s/%s" % (fam, name), fill=(245, 240, 225, 255))
    os.makedirs(os.path.dirname(a.out), exist_ok=True)
    sheet.convert("RGB").save(a.out)
    print(a.out, len(items))


if __name__ == "__main__":
    main()
