#!/usr/bin/env python3
"""Storyteller portraits (ART §17.1) and the main-menu hero: raster source -> UI art.

The busts are painted (AI-assisted, see source/raster/ui/portraits/briefs.md)
on a flat magenta field. This keys the field to alpha, turns the magenta
fringe into outline ink, fits the bust bottom-flush into the 512x640 canvas,
adds a dark contour, cuts a 120x150 head-and-shoulders thumbnail, and writes
the sidecar metadata. Deterministic: the same source gives the same bytes.

    python3 assets/source/tools/portrait_cut.py
"""
import os

import numpy as np
from PIL import Image, ImageFilter

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SRC = os.path.join(ROOT, "source", "raster", "ui", "portraits")
OUT = os.path.join(ROOT, "generated", "ui", "portraits")
META = os.path.join(ROOT, "metadata", "ui_portrait")

CANVAS = (512, 640)
THUMB = (120, 150)
THUMB_WIDTH = 0.72  # thumbnail window width as a share of the canvas
HEAD_SHARE = 0.30  # top share of the bust that locates the head
INK = np.array([21, 17, 14], dtype=np.float32)  # outline ink, matches the menu art
KEY = np.array([255, 0, 255], dtype=np.float32)
KEY_LO, KEY_HI = 110.0, 190.0  # RGB distance to magenta: fully keyed .. fully kept
SPILL_BAND = 6  # px from the keyed edge in which magenta spill is removed
CONTOUR, THUMB_CONTOUR = 4, 2

MENU_SRC = os.path.join(ROOT, "source", "raster", "ui", "menu", "hero.png")
MENU_OUT = os.path.join(ROOT, "source", "raster", "ui", "main_menu_colonist.png")  # read by splash.go
MENU_CANVAS = (904, 1104)

PORTRAITS = {  # storyteller key -> character (internal/domain/storyteller)
    "steady": "Orla Vane",
    "hearth": "Bram Holloway",
    "gambler": "Kestrel Quay",
}


def key_out(img):
    """Magenta field -> alpha; the soft fringe is pulled to outline ink.

    Pixels near the keyed edge lose their magenta spill (min(R, B) above G),
    so compression noise and see-through steam don't leave a pink halo.
    """
    rgb = np.asarray(img.convert("RGB"), dtype=np.float32)
    d = np.sqrt(((rgb - KEY) ** 2).sum(axis=2))
    t = np.clip((d - KEY_LO) / (KEY_HI - KEY_LO), 0.0, 1.0)
    a = t * t * (3.0 - 2.0 * t)
    near = np.asarray(Image.fromarray(((a < 0.99) * 255).astype(np.uint8))
                      .filter(ImageFilter.MaxFilter(2 * SPILL_BAND + 1)), dtype=np.float32) / 255.0
    spill = np.clip(np.minimum(rgb[..., 0], rgb[..., 2]) - rgb[..., 1], 0.0, None) * near
    out = rgb.copy()
    out[..., 0] -= spill
    out[..., 2] -= spill
    edge = (1.0 - a)[..., None]
    out = out + (INK - out) * (edge * 0.85)
    rgba = np.dstack([np.clip(out, 0, 255), a * 255.0]).round().astype(np.uint8)
    return Image.fromarray(rgba, "RGBA")


def fit(img, size):
    """Trim to the opaque bounds, scale to fit, centre, flush to the bottom."""
    box = img.getchannel("A").point(lambda v: 255 if v > 8 else 0).getbbox()
    img = img.crop(box)
    s = min(size[0] / img.width, size[1] / img.height)
    w, h = max(1, round(img.width * s)), max(1, round(img.height * s))
    img = img.resize((w, h), Image.LANCZOS)
    canvas = Image.new("RGBA", size, (0, 0, 0, 0))
    canvas.alpha_composite(img, ((size[0] - w) // 2, size[1] - h))
    return canvas


SIDES = ("left", "top", "right", "bottom")


def clipped_sides(img):
    """The sides of img that opaque content runs off: the painting was cut
    there, so no contour belongs on that edge (it would draw a straight
    dark line along the cut)."""
    a = np.asarray(img.getchannel("A")) > 8
    if not a.any():
        return set()
    out = set()
    if a[:, 0].any():
        out.add("left")
    if a[0, :].any():
        out.add("top")
    if a[:, -1].any():
        out.add("right")
    if a[-1, :].any():
        out.add("bottom")
    return out


def contour(img, width, clipped=()):
    """A dark outer contour `width` px wide around the opaque shape, except
    past the shape's edge on a side listed in `clipped` (see clipped_sides):
    a bust is cut flush at the bottom by design, and a source cut on a side
    keeps a clean edge there instead of an inked band."""
    a = img.getchannel("A")
    grown = a.filter(ImageFilter.MaxFilter(2 * width + 1))
    if clipped:
        box = a.point(lambda v: 255 if v > 8 else 0).getbbox()
        if box is not None:
            g, raw = np.asarray(grown).copy(), np.asarray(a)
            x0, y0, x1, y1 = box
            if "left" in clipped:
                g[:, :x0] = raw[:, :x0]
            if "top" in clipped:
                g[:y0, :] = raw[:y0, :]
            if "right" in clipped:
                g[:, x1:] = raw[:, x1:]
            if "bottom" in clipped:
                g[y1:, :] = raw[y1:, :]
            grown = Image.fromarray(g, "L")
    ring = Image.new("RGBA", img.size, tuple(int(v) for v in INK) + (0,))
    ring.putalpha(grown)
    ring.alpha_composite(img)
    return ring


def thumbnail(full):
    """Head and shoulders: a THUMB-shaped window over the head, scaled down.

    The head is the opaque mass of the top HEAD_SHARE of the bust; the window
    is THUMB_WIDTH of the canvas wide, centred on it, starting just above it.
    """
    a = np.asarray(full.getchannel("A"), dtype=np.float32)
    top = int(np.argmax(a.max(axis=1) > 128))
    band = a[top: top + round(full.height * HEAD_SHARE)]
    xs = np.nonzero(band.max(axis=0) > 128)[0]
    cx = (xs[0] + xs[-1]) / 2.0 if len(xs) else full.width / 2.0
    w = round(full.width * THUMB_WIDTH)
    h = round(w * THUMB[1] / THUMB[0])
    x0 = int(min(max(cx - w / 2.0, 0), full.width - w))
    y0 = max(top - 8, 0)
    win = full.crop((x0, y0, x0 + w, y0 + h))
    inner = win.resize((THUMB[0] - 2 * THUMB_CONTOUR, THUMB[1] - THUMB_CONTOUR), Image.LANCZOS)
    canvas = Image.new("RGBA", THUMB, (0, 0, 0, 0))
    canvas.alpha_composite(inner, (THUMB_CONTOUR, THUMB_CONTOUR))
    return contour(canvas, THUMB_CONTOUR)


def _fit_exact(img, size, pad):
    """fit() into size, leaving `pad` px for the contour on the sides and top."""
    inner = fit(img, (size[0] - 2 * pad, size[1] - pad))
    canvas = Image.new("RGBA", size, (0, 0, 0, 0))
    canvas.alpha_composite(inner, (pad, pad))
    return canvas


def write_meta(name, lines):
    with open(os.path.join(META, name + ".yaml"), "w", encoding="utf-8") as f:
        f.write("# generated by assets/source/tools/portrait_cut.py - do not hand-edit\n")
        f.write("".join(line + "\n" for line in lines))


def cut(key, character):
    src = Image.open(os.path.join(SRC, key + ".png"))
    keyed = key_out(src)
    full = contour(_fit_exact(keyed, CANVAS, CONTOUR), CONTOUR, clipped_sides(keyed) | {"bottom"})
    thumb = thumbnail(full)
    full.save(os.path.join(OUT, key + ".png"), optimize=True)
    thumb.save(os.path.join(OUT, key + "_thumb.png"), optimize=True)
    write_meta(key, [
        "version: 1",
        'id: "core:ui_portrait/%s"' % key,
        "image: generated/ui/portraits/%s.png" % key,
        "scale: 2",
        "canvas: [%d, %d]" % CANVAS,
        "footprint: [4, 5]",
        "origin_px: [0, 0]",
        "shadow: {kind: none, w: 0.0, h: 0.0, d: 0.0}",
        "portrait: %s" % key,
        "character: %s" % character,
        "thumbnail: %s_thumb" % key,
        "source: source/raster/ui/portraits/%s.png" % key,
    ])
    write_meta(key + "_thumb", [
        "version: 1",
        'id: "core:ui_portrait/%s_thumb"' % key,
        "image: generated/ui/portraits/%s_thumb.png" % key,
        "scale: 1",
        "canvas: [%d, %d]" % THUMB,
        "footprint: [0, 0]",
        "origin_px: [0, 0]",
        "shadow: {kind: none, w: 0.0, h: 0.0, d: 0.0}",
        "portrait: %s" % key,
        "character: %s" % character,
        "thumbnail: true",
        "source: generated/ui/portraits/%s.png" % key,
    ])


def cut_menu():
    """The main-menu hero (ART-028): keyed and inked like the busts, fitted
    bottom-flush into MENU_CANVAS with a contour; no thumbnail or metadata
    (splash.go loads the PNG directly).

    The splash sits bottom-left of the window, so the figure may run off the
    left and bottom edges but never the right or top (ART-029): a figure cut
    there shows as a straight edge against the backdrop. A source like that
    is refused; paint or extend it with magenta margin instead.
    """
    keyed = key_out(Image.open(MENU_SRC))
    sides = clipped_sides(keyed)
    if bad := sides & {"right", "top"}:
        raise SystemExit("%s: the figure runs off the %s edge; the menu shows that edge, so "
                         "regenerate the source with magenta margin there (ART-029)"
                         % (os.path.relpath(MENU_SRC, ROOT), " and ".join(sorted(bad))))
    contour(_fit_exact(keyed, MENU_CANVAS, CONTOUR), CONTOUR, sides | {"bottom"}).save(MENU_OUT, optimize=True)


def main():
    os.makedirs(OUT, exist_ok=True)
    os.makedirs(META, exist_ok=True)
    for key, character in PORTRAITS.items():
        cut(key, character)
        print("portrait", key)
    cut_menu()
    print("menu hero")


if __name__ == "__main__":
    main()
