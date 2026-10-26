#!/usr/bin/env python3
"""Muted, warmer palette pass (ART §7, §33; RimWorld-look art pass).

    python3 assets/source/tools/palette_mute.py [--out DIR] [--sat 0.8] [--contrast 0.7]
        [--light 0.96] [--warm 8] [--outline 0.18,3] [--tints] [--force]

Rewrites the three toon bands of every material in materials.yaml and
materials_*.yaml:

  1. band contrast: light and shadow move toward base (x --contrast);
  2. saturation x --sat (HSV) on every band;
  3. light band value x --light (a little less bright);
  4. shadow hue turned up to --warm degrees toward warm orange (30 deg).

Skipped: the emissive: section, materials matching --skip (glows, fire,
signal colours), and tint: true neutral greys (the runtime tint colours
them; --tints mutes the base.yaml tint presets instead, never stone_tint).
--outline D,W sets base.yaml outline {darken: D, width_px_1x: W}.

Deterministic, and guarded: a processed file gets a `# palette_mute:` header
line and is never processed twice (--force overrides). With --out every
palette file is written to DIR (unprocessed ones copied), so DIR can be fed
to a review render as RWG_PALETTE_DIR; without it the files change in place.
"""
import argparse
import colorsys
import glob
import os
import re
import shutil
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PALETTES = os.path.join(ROOT, "palettes")
MARK = "# palette_mute:"
ENTRY = re.compile(r'^(\s{2}[a-z_]+:\s*\{)(.*)(\}\s*)$')
HEX = re.compile(r'(light|base|shadow):\s*"(#[0-9a-fA-F]{6})"')
TINT_LINE = re.compile(r'^(\s+)(skin|hair|cloth):(\s*)\{(.*)\}(.*)$')
WARM_HUE = 30.0 / 360.0
DEFAULT_SKIP = r"glow|^fire_|ember|^indicator$|^med_|^sign_paint$"


def hex_rgb(h):
    h = h.lstrip("#")
    return [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]


def rgb_hex(c):
    return "#" + "".join("%02x" % int(round(min(1.0, max(0.0, v)) * 255)) for v in c)


def sat_of(c):
    return colorsys.rgb_to_hsv(*c)[1]


def toward_hue(h, target, max_step):
    d = (target - h + 0.5) % 1.0 - 0.5          # shortest signed arc
    return (h + max(-max_step, min(max_step, d))) % 1.0


def mute_bands(bands, a):
    """bands: {light, base, shadow} sRGB floats -> muted copy."""
    base = bands["base"]
    out = {"base": list(base)}
    for k, keep in (("light", a.contrast), ("shadow", a.contrast)):
        out[k] = [b + (v - b) * keep for v, b in zip(bands[k], base)]
    for k in out:
        h, s, v = colorsys.rgb_to_hsv(*out[k])
        s *= a.sat
        if k == "light":
            v *= a.light
        if k == "shadow" and a.warm:
            h = toward_hue(h, WARM_HUE, a.warm / 360.0)
        out[k] = list(colorsys.hsv_to_rgb(h, s, v))
    return out


def is_neutral(bands):
    return all(sat_of(c) < 0.04 for c in bands.values())


def mark_line(a, what):
    return "%s applied %s (sat=%g contrast=%g light=%g warm=%g)\n" % (MARK, what, a.sat, a.contrast, a.light, a.warm)


def insert_mark(lines, mark):
    i = 0
    while i < len(lines) and lines[i].startswith("#"):
        i += 1
    return lines[:i] + [mark] + lines[i:]


def mute_materials(text, a, skip):
    lines = text.splitlines(keepends=True)
    section, out, n = None, [], 0
    for line in lines:
        if line.startswith("materials:"):
            section = "m"
        elif line.startswith("emissive:"):
            section = "e"
        elif line and not line[0].isspace() and not line.startswith("#"):
            section = None
        m = ENTRY.match(line.rstrip("\n")) if section == "m" else None
        if m:
            name = line.split(":")[0].strip()
            body = m.group(2)
            bands = {k: hex_rgb(v) for k, v in HEX.findall(body)}
            tint = re.search(r"tint:\s*true", body) is not None
            if len(bands) == 3 and not skip.search(name) and not (tint and is_neutral(bands)):
                new = mute_bands(bands, a)
                body = HEX.sub(lambda mm: '%s: "%s"' % (mm.group(1), rgb_hex(new[mm.group(1)])), body)
                line = m.group(1) + body + m.group(3) + ("\n" if line.endswith("\n") else "")
                n += 1
        out.append(line)
    return "".join(insert_mark(out, mark_line(a, "materials"))), n


def mute_base(text, a):
    """base.yaml: optional outline {darken, width} and muted tint presets."""
    lines = text.splitlines(keepends=True)
    out, in_outline = [], False
    for line in lines:
        if line.startswith("outline:"):
            in_outline = True
        elif line and not line[0].isspace():
            in_outline = False
        if in_outline and a.outline:
            d, w = a.outline
            line = re.sub(r"^(\s+darken:\s*)[\d.]+", lambda m: m.group(1) + ("%g" % d), line)
            line = re.sub(r"^(\s+width_px_1x:\s*)[\d.]+", lambda m: m.group(1) + ("%g" % w), line)
        if a.tints:
            m = TINT_LINE.match(line.rstrip("\n"))
            if m:
                def one(mm):
                    h, s, v = colorsys.rgb_to_hsv(*hex_rgb(mm.group(2)))
                    return '%s: "%s"' % (mm.group(1), rgb_hex(colorsys.hsv_to_rgb(h, s * a.sat, v)))
                body = re.sub(r'(\w+):\s*"(#[0-9a-fA-F]{6})"', one, m.group(4))
                line = "%s%s:%s{%s}%s\n" % (m.group(1), m.group(2), m.group(3), body, m.group(5))
        out.append(line)
    return "".join(out)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--src", default=PALETTES, help="palette dir to read (default assets/palettes)")
    ap.add_argument("--out", help="write the palette set here instead of in place")
    ap.add_argument("--sat", type=float, default=0.8, help="HSV saturation multiplier")
    ap.add_argument("--contrast", type=float, default=0.7, help="light/shadow distance from base multiplier")
    ap.add_argument("--light", type=float, default=0.96, help="light band value multiplier")
    ap.add_argument("--warm", type=float, default=8.0, help="max shadow hue turn toward 30 deg, degrees")
    ap.add_argument("--skip", default=DEFAULT_SKIP, help="regex of material names left alone")
    ap.add_argument("--outline", help="base.yaml outline darken,width_px_1x (e.g. 0.18,3)")
    ap.add_argument("--tints", action="store_true", help="also mute base.yaml skin/hair/cloth presets (x --sat)")
    ap.add_argument("--force", action="store_true", help="process files already marked")
    a = ap.parse_args()
    a.outline = tuple(float(x) for x in a.outline.split(",")) if a.outline else None
    skip = re.compile(a.skip)
    dst = a.out or a.src
    os.makedirs(dst, exist_ok=True)
    if a.out:
        for f in glob.glob(os.path.join(a.src, "*.yaml")):
            shutil.copyfile(f, os.path.join(dst, os.path.basename(f)))
    total = 0
    for f in sorted(glob.glob(os.path.join(dst, "materials*.yaml"))):
        text = open(f, encoding="utf-8").read()
        if MARK in text and not a.force:
            print("%s: already muted, skipped" % os.path.basename(f))
            continue
        text, n = mute_materials(text, a, skip)
        open(f, "w", encoding="utf-8").write(text)
        total += n
        print("%s: %d materials muted" % (os.path.basename(f), n))
    base = os.path.join(dst, "base.yaml")
    if a.outline or a.tints:
        text = open(base, encoding="utf-8").read()
        if a.tints and MARK in text and not a.force:
            print("base.yaml: tints already muted; outline only")
            a.tints = False
        new = mute_base(text, a)
        if a.tints:
            new = "".join(insert_mark(new.splitlines(keepends=True), mark_line(a, "tints")))
        open(base, "w", encoding="utf-8").write(new)
        print("base.yaml: outline=%s tints=%s" % (a.outline, a.tints))
    print("muted %d materials into %s" % (total, dst))
    return 0


if __name__ == "__main__":
    sys.exit(main())
