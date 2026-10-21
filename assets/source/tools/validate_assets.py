#!/usr/bin/env python3
"""Asset validator (ART spec §25, §29). Exit 1 on any error.

Checks every generated sprite against its sidecar metadata: presence,
id/naming, canvas size, origin, footprint, @2x scale, tint masks, alpha,
tileable terrain seams, wall autotile completeness, and pawn layer
canvas agreement.

    python3 assets/source/tools/validate_assets.py [--no-light]

Every sprite must also have a normal map <name>_n.png of the same size
and alpha (make normals) and a shadow: block in its metadata (ART §6.1);
--no-light skips those two checks.
"""
import os
import re
import sys

from PIL import Image

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
GEN = os.path.join(ROOT, "generated", "world")
META = os.path.join(ROOT, "metadata")
PPT = 128
ID = re.compile(r"^core:[a-z_]+/[a-z0-9_]+$")

errors = []
LIGHT = "--no-light" not in sys.argv[1:]
SHADOW_KINDS = ("blob", "box", "none")


def err(msg):
    errors.append(msg)


def field(md, key):
    m = re.search(r"^%s: (.+)$" % key, md, re.M)
    return m.group(1).strip().strip('"') if m else None


def ints(v):
    return [int(float(x)) for x in re.findall(r"-?[\d.]+", v or "")]


def check_sprite(family, name, path):
    mp = os.path.join(META, family, name + ".yaml")
    if not os.path.exists(mp):
        return err("%s/%s: missing metadata" % (family, name))
    md = open(mp, encoding="utf-8").read()
    if field(md, "id") != "core:%s/%s" % (family, name) or not ID.match(field(md, "id") or ""):
        err("%s/%s: bad id %r" % (family, name, field(md, "id")))
    if not re.match(r"^[a-z0-9_]+$", name):
        err("%s/%s: name must be lowercase snake_case" % (family, name))
    img = Image.open(path)
    if img.mode != "RGBA":
        err("%s/%s: not RGBA" % (family, name))
    cw, ch = ints(field(md, "canvas"))
    if (cw, ch) != img.size:
        err("%s/%s: canvas %s != png %s" % (family, name, (cw, ch), img.size))
    if field(md, "scale") != "2":
        err("%s/%s: scale must be 2" % (family, name))
    fw, fh = ints(field(md, "footprint"))
    ox, oy = ints(field(md, "origin_px"))
    if ox < 0 or oy < 0 or ox + fw * PPT > cw or oy + fh * PPT > ch:
        err("%s/%s: footprint %dx%d at %s exceeds canvas" % (family, name, fw, fh, (ox, oy)))
    a = img.getchannel("A")
    if a.getextrema()[1] == 0:
        err("%s/%s: fully transparent" % (family, name))
    if "tint:" in md:
        mask = re.search(r"mask: (\S+)", md).group(1)
        mpth = os.path.join(ROOT, mask)
        if not os.path.exists(mpth):
            err("%s/%s: tint mask missing" % (family, name))
        elif Image.open(mpth).size != img.size:
            err("%s/%s: tint mask size mismatch" % (family, name))
        g = re.search(r"group: (\w+)", md).group(1)
        if g not in ("cloth", "skin", "hair"):
            err("%s/%s: unknown tint group %s" % (family, name, g))
    if LIGHT:
        check_light(family, name, path, md, img)
    return md


def check_light(family, name, path, md, img):
    """The normal map and shadow caster of one sprite (ART §6.1)."""
    npath = path[:-4] + "_n.png"
    if not os.path.exists(npath):
        err("%s/%s: missing normal map (make normals)" % (family, name))
    else:
        n = Image.open(npath)
        if n.size != img.size:
            err("%s/%s: normal map size %s != %s" % (family, name, n.size, img.size))
        elif n.convert("RGBA").getchannel("A").tobytes() != img.getchannel("A").tobytes():
            err("%s/%s: normal map alpha differs from the sprite" % (family, name))
    m = re.search(r"^shadow:\n  kind: (\w+)\n  w: ([\d.]+)\n  h: ([\d.]+)\n  d: ([\d.]+)$", md, re.M)
    if not m:
        err("%s/%s: missing shadow: block (kind, w, h, d)" % (family, name))
    elif m.group(1) not in SHADOW_KINDS:
        err("%s/%s: unknown shadow kind %s" % (family, name, m.group(1)))


def check_terrain_seams():
    groups = {}
    for f in os.listdir(os.path.join(GEN, "terrain")):
        m = re.match(r"^(\w+)_(\d\d)\.png$", f)
        mp = os.path.join(META, "terrain", f[:-4] + ".yaml")
        if m and os.path.exists(mp) and "tileable: true" in open(mp).read():
            groups.setdefault(m.group(1), []).append(f)
    for t, files in groups.items():
        imgs = [Image.open(os.path.join(GEN, "terrain", f)).convert("RGB") for f in sorted(files)]
        for im in imgs:
            if im.size != (PPT, PPT):
                err("terrain/%s: tile must be %dx%d" % (t, PPT, PPT))
            if im.convert("RGBA").getchannel("A").getextrema()[0] != 255 if im.mode == "RGBA" else False:
                err("terrain/%s: tiles must be opaque" % t)
        # any variant's east edge must continue into any variant's west edge
        for a in imgs:
            for b in imgs:
                ea = [a.getpixel((PPT - 1, y)) for y in range(PPT)]
                wb = [b.getpixel((0, y)) for y in range(PPT)]
                diff = sum(sum(abs(p - q) for p, q in zip(u, v)) for u, v in zip(ea, wb)) / (PPT * 3)
                if diff > 22:
                    err("terrain/%s: E/W seam too visible between variants (%.1f)" % (t, diff))
                    break


def check_autotile():
    for mat in ("wood",):
        missing = [m for m in range(16)
                   if not os.path.exists(os.path.join(GEN, "structure", "wall_%s_%02d.png" % (mat, m)))]
        if missing:
            err("structure/wall_%s: missing masks %s" % (mat, missing))


# ART §27.1: one 12-pose gait cycle, a 6-pose work swing, an 8-pose trot.
WALK_FRAMES = tuple("walk%02d" % i for i in range(1, 13))
WORK_FRAMES = tuple("work%d" % i for i in range(1, 7))
TROT_FRAMES = tuple("walk%02d" % i for i in range(1, 9))
ANIM_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility", "parka_leather", "duster_leather")


def _set(name, n):
    return tuple("%s%d" % (name, i) for i in range(1, n + 1))


# ART §27.2: action pose sets on every animated layer.
ACTION_FRAMES = (("ready", "aim", "kneel", "carry") + _set("fire", 3) + _set("stab", 6) +
                 _set("thrust", 6) + _set("swing", 6) + _set("punch", 4) + _set("chop", 6) +
                 _set("mine", 6) + _set("hammer", 4) + _set("plant", 4) + _set("pick", 4) +
                 _set("bench", 4) + _set("tend", 4) + _set("eat", 4) + _set("flinch", 3) +
                 _set("fall", 4) + _set("offer", 2) +
                 tuple("carry_walk%02d" % i for i in range(1, 13)))
ANIMAL_ACTIONS = {"dustbuck": _set("kick", 4), "ridgewolf": _set("bite", 4)}
ANIMAL_COMMON = _set("graze", 2) + _set("flinch", 2)
# held gear: two-handed items need a support point towards the working end
GEAR_END = {"weapon": ("muzzle", "tip"), "tool": ("head",)}
# held level in the off hand, not pointed along an axis
GEAR_LEVEL = {"tool/bowl"}


def check_pawn_layers():
    d = os.path.join(GEN, "pawn")
    sizes = {}
    for f in os.listdir(d):
        m = re.match(r"^(\w+?)_(south|east|north|west)(?:_[a-z_]+\d*)?\.png$", f)
        if m and not f.endswith("_mask.png") and not f.endswith("_n.png"):
            sizes.setdefault(m.group(2), set()).add(Image.open(os.path.join(d, f)).size)
    # every animated layer has the full walk cycle (frames share a canvas)
    for layer in ANIM_LAYERS:
        for facing in ("south", "east", "north", "west"):
            for fr in WALK_FRAMES:
                if not os.path.exists(os.path.join(d, "%s_%s_%s.png" % (layer, facing, fr))):
                    err("pawn/%s_%s: missing walk frame %s" % (layer, facing, fr))
            for fr in WORK_FRAMES:
                if not os.path.exists(os.path.join(d, "%s_%s_%s.png" % (layer, facing, fr))):
                    err("pawn/%s_%s: missing work frame %s" % (layer, facing, fr))
            for fr in ACTION_FRAMES:
                if not os.path.exists(os.path.join(d, "%s_%s_%s.png" % (layer, facing, fr))):
                    err("pawn/%s_%s: missing action frame %s" % (layer, facing, fr))
    # hair and hats follow the body's head anchor (ART §27.1)
    md = os.path.join(META, "pawn")
    for f in sorted(os.listdir(md)) if os.path.isdir(md) else ():
        if f.startswith("body_") and f.endswith(".yaml"):
            with open(os.path.join(md, f)) as fh:
                text = fh.read()
                for key in ("head", "hand", "hand_l"):
                    if not re.search(r"^\s+%s:\s*\[" % key, text, re.M):
                        err("pawn/%s: missing %s attachment" % (f[:-5], key))
    ad = os.path.join(GEN, "animal")
    if os.path.isdir(ad):
        for f in os.listdir(ad):
            m = re.match(r"^(\w+?)_(south|east|north|west)\.png$", f)
            if not m:
                continue
            base = Image.open(os.path.join(ad, f)).size
            for fr in TROT_FRAMES:
                p = os.path.join(ad, "%s_%s_%s.png" % (m.group(1), m.group(2), fr))
                if not os.path.exists(p):
                    err("animal/%s: missing trot frame %s" % (f[:-4], fr))
                elif Image.open(p).size != base:
                    err("animal/%s_%s: canvas differs from the standing frame" % (f[:-4], fr))
            for fr in ANIMAL_ACTIONS.get(m.group(1), ()) + ANIMAL_COMMON:
                p = os.path.join(ad, "%s_%s_%s.png" % (m.group(1), m.group(2), fr))
                if not os.path.exists(p):
                    err("animal/%s: missing action frame %s" % (f[:-4], fr))
                elif Image.open(p).size != base:
                    err("animal/%s_%s: canvas differs from the standing frame" % (f[:-4], fr))
    for facing, s in sizes.items():
        if len(s) != 1:
            err("pawn/%s: layers disagree on canvas %s" % (facing, sorted(s)))
    if set(sizes) != {"south", "east", "north", "west"}:
        err("pawn: need all four facings, got %s" % sorted(sizes))


def _attach(text, key):
    m = re.search(r"^\s+%s:\s*\[([-\d.]+),\s*([-\d.]+)\]" % key, text, re.M)
    return (float(m.group(1)), float(m.group(2))) if m else None


def check_gear():
    """Rotatable held gear points east: grip, then support, then the
    working end (ART §15, §27.2)."""
    for family, ends in GEAR_END.items():
        md = os.path.join(META, family)
        for f in sorted(os.listdir(md)) if os.path.isdir(md) else ():
            text = open(os.path.join(md, f)).read()
            if not re.search(r"^rotatable:\s*true", text, re.M):
                continue
            name = "%s/%s" % (family, f[:-5])
            if name in GEAR_LEVEL:
                continue
            grip, sup = _attach(text, "grip"), _attach(text, "support")
            end = next((_attach(text, k) for k in ends if _attach(text, k)), None)
            if not grip or not sup or not end:
                err("%s: needs grip, support and one of %s" % (name, "/".join(ends)))
                continue
            if not grip[0] < sup[0] <= end[0]:
                err("%s: support must lie between grip and the working end (x %.0f, %.0f, %.0f)"
                    % (name, grip[0], sup[0], end[0]))


def main():
    count = 0
    for family in sorted(os.listdir(GEN)):
        d = os.path.join(GEN, family)
        for f in sorted(os.listdir(d)):
            if f.endswith(".png") and not f.endswith("_mask.png") and not f.endswith("_n.png"):
                check_sprite(family, f[:-4], os.path.join(d, f))
                count += 1
    for family in sorted(os.listdir(META)):
        for f in os.listdir(os.path.join(META, family)):
            img = re.search(r"^image: (\S+)", open(os.path.join(META, family, f)).read(), re.M)
            if img and not os.path.exists(os.path.join(ROOT, img.group(1))):
                err("%s/%s: metadata points at missing image %s" % (family, f, img.group(1)))
    check_terrain_seams()
    check_autotile()
    check_pawn_layers()
    check_gear()
    for e in errors:
        print("ERROR", e)
    print("%d sprites checked, %d errors" % (count, len(errors)))
    sys.exit(1 if errors else 0)


if __name__ == "__main__":
    main()
