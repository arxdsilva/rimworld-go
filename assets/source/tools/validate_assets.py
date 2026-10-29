#!/usr/bin/env python3
"""Asset validator (ART spec §25, §29). Exit 1 on any error.

Checks every generated sprite against its sidecar metadata: presence,
id/naming, canvas size, origin, footprint, @2x scale, tint masks, alpha,
tileable terrain seams, wall autotile completeness, and the pawn layer
contract (one standing sprite per layer and facing, attachments, canvas).

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


def tint_groups():
    """Tint group names declared under `tints:` in palettes/base.yaml."""
    out, inside = set(), False
    for line in open(os.path.join(ROOT, "palettes", "base.yaml"), encoding="utf-8"):
        if not line.startswith(" "):
            inside = line.startswith("tints:")
            continue
        m = re.match(r"^\s{2}(\w+):\s*\{", line)
        if inside and m:
            out.add(m.group(1))
    return out


TINT_GROUPS = tint_groups()
PPT = 128
# PLAY-022: animals and corpses carry their own density `ppt` (pixels per
# tile, 64..128; the renderer scales them to the tile) and their canvas may
# be tighter than the tile, so origin_px may lie outside it.
MIN_PPT = 64
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
    ppt = field(md, "ppt")
    if ppt is None:
        if ox < 0 or oy < 0 or ox + fw * PPT > cw or oy + fh * PPT > ch:
            err("%s/%s: footprint %dx%d at %s exceeds canvas" % (family, name, fw, fh, (ox, oy)))
    elif not ppt.isdigit() or not MIN_PPT <= int(ppt) <= PPT:
        err("%s/%s: ppt %s outside %d..%d" % (family, name, ppt, MIN_PPT, PPT))
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
        if g not in TINT_GROUPS:
            err("%s/%s: unknown tint group %s" % (family, name, g))
    if LIGHT:
        check_light(family, name, path, md, img)
    return md


def check_light(family, name, path, md, img):
    """The normal map and shadow caster of one sprite (ART §6.1)."""
    npath = path[:-4] + "_n.png"
    # terrain, floors and the flat ground decals (snow cover, frost; PLAY-022)
    flat = family == "terrain" or (family == "structure" and name.startswith("floor")) or \
        (family == "weather" and name.startswith(("snow_cover", "frost")))
    if flat:
        if os.path.exists(npath):
            err("%s/%s: terrain, floors and ground decals are lit flat; remove the normal map" % (family, name))
    elif not os.path.exists(npath):
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


# ART §27.1: an 8-pose animal trot. Pawns have no frames (ART §10, §27.1-27.2): one
# standing sprite per layer and facing; the renderer moves, mirrors and rotates it.
TROT_FRAMES = tuple("walk%02d" % i for i in range(1, 9))
ANIMAL_FACINGS = ("south", "east", "north")    # west is east mirrored by the renderer
# Per-build layers (families/pawn.py BUILD_LAYERS): m_avg keeps the un-suffixed keys,
# the rest are <layer>_<build>_<facing>. Hair, beards, the tuque and the scar are shared.
BUILD_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility", "parka_leather", "duster_leather",
                "flak_pants", "flak_vest", "flak_jacket", "plate_armor", "marine_armor")
PAWN_BUILDS = ("m_thin", "m_avg", "m_heavy", "f_thin", "f_avg", "f_heavy")
LEGACY_BUILD = "m_avg"
HAIR_LAYERS = ("hair_short", "hair_crop", "hair_bun", "hair_long", "hair_braid", "hair_shaved", "hair_curls",
               "hair_tail", "beard_short", "beard_full", "beard_moustache")
SHARED_LAYERS = HAIR_LAYERS + ("tuque_knit", "face_scar", "simple_helmet", "advanced_helmet", "marine_helmet")
PAWN_FACINGS = ("south", "east", "north")      # west is east mirrored by the renderer
BODY_ATTACH = ("head", "hold", "back")
GONE_ATTACH = ("hand", "hand_l")
ATLAS_META = os.path.join(META, "atlas")
ATLAS_GEN = os.path.join(ROOT, "generated", "atlas")
ATLAS_MAX = 4096


def _set(name, n):
    return tuple("%s%d" % (name, i) for i in range(1, n + 1))


ANIMAL_COMMON = _set("graze", 2) + _set("flinch", 2)
# held gear: two-handed items need a support point towards the working end
GEAR_END = {"weapon": ("muzzle", "tip")}


def parse_atlas(path):
    """metadata/atlas/<family>.yaml (cmd/atlasgen): header fields and, per sprite,
    page, rect [x, y, w, h], trim, normal, canvas, tint mask and attachments."""
    out = {"family": None, "max": 0, "pages": 0, "sprites": {}}
    cur, parent, in_sprites = None, None, False
    for line in open(path, encoding="utf-8"):
        line = line.rstrip()
        if not line or line.lstrip().startswith("#"):
            continue
        indent = len(line) - len(line.lstrip(" "))
        k, _, v = line.strip().partition(":")
        v = v.strip().strip('"')
        if indent == 0 and not in_sprites:
            if k == "family":
                out["family"] = v
            elif k in ("max", "pages"):
                out[k] = int(v)
            elif k == "sprites":
                in_sprites = True
        elif in_sprites and indent == 2:
            cur = out["sprites"][k] = {"attach": {}, "mask": False}
        elif in_sprites and cur is not None and indent == 4:
            parent = k if not v else None
            if k == "page":
                cur["page"] = int(v)
            elif k in ("rect", "trim", "canvas"):
                cur[k] = ints(v)
            elif k == "normal":
                cur["normal"] = v == "true"
            elif k == "layer":
                cur["layer"] = v
        elif in_sprites and cur is not None and indent == 6:
            if parent == "tint" and k == "mask":
                cur["mask"] = True
            elif parent == "attachments":
                cur["attach"][k] = ints(v)
    return out


def check_atlas():
    """Atlas integrity (NG-1 plan 7.1): pages exist and fit, rects lie inside their page
    without overlapping, mask/normal pages match their sprite page, every sprite that
    still has individual metadata is in the atlas. Returns {family: {key: entry}}."""
    atlases = {}
    if not os.path.isdir(ATLAS_META):
        return atlases
    for f in sorted(os.listdir(ATLAS_META)):
        if not f.endswith(".yaml"):
            continue
        fam = f[:-5]
        a = parse_atlas(os.path.join(ATLAS_META, f))
        atlases[fam] = a["sprites"]
        where = "atlas/%s" % fam
        if a["family"] != fam:
            err("%s: family field is %r" % (where, a["family"]))
        if not a["sprites"]:
            err("%s: no sprites" % where)
        sizes = []
        for p in range(a["pages"]):
            path = os.path.join(ATLAS_GEN, "%s_%d.png" % (fam, p))
            if not os.path.exists(path):
                err("%s: missing page %d" % (where, p))
                sizes.append((0, 0))
                continue
            sizes.append(Image.open(path).size)
            if max(sizes[-1]) > min(a["max"] or ATLAS_MAX, ATLAS_MAX):
                err("%s: page %d is %s, above %d" % (where, p, sizes[-1], ATLAS_MAX))
        by_page = {}
        for key, s in a["sprites"].items():
            if "page" not in s or "rect" not in s or "canvas" not in s:
                err("%s/%s: incomplete atlas entry" % (where, key))
                continue
            p, (x, y, w, h) = s["page"], s["rect"]
            if not 0 <= p < len(sizes):
                err("%s/%s: page %d out of range" % (where, key, p))
                continue
            pw, ph = sizes[p]
            if w < 1 or h < 1 or x < 0 or y < 0 or x + w > pw or y + h > ph:
                err("%s/%s: rect %s outside page %d (%dx%d)" % (where, key, s["rect"], p, pw, ph))
            tx, ty = s.get("trim", (0, 0))
            if tx < 0 or ty < 0 or tx + w > s["canvas"][0] or ty + h > s["canvas"][1]:
                err("%s/%s: trim %s + %dx%d exceeds canvas %s" % (where, key, s.get("trim"), w, h, s["canvas"]))
            by_page.setdefault(p, []).append((x, y, w, h, key))
        for p, rs in by_page.items():
            rs.sort()
            for i, (x, y, w, h, key) in enumerate(rs):
                for x2, y2, w2, h2, key2 in rs[i + 1:]:
                    if x2 >= x + w:
                        break
                    if y2 < y + h and y < y2 + h2:
                        err("%s: page %d: %s and %s overlap" % (where, p, key, key2))
        for p, sz in enumerate(sizes):
            for suffix, need in (("_mask", any(s["mask"] and s.get("page") == p for s in a["sprites"].values())),
                                 ("_n", any(s.get("normal") and s.get("page") == p for s in a["sprites"].values()))):
                path = os.path.join(ATLAS_GEN, "%s_%d%s.png" % (fam, p, suffix))
                if need and not os.path.exists(path):
                    err("%s: missing page file %s_%d%s.png" % (where, fam, p, suffix))
                elif need and Image.open(path).size != sz:
                    err("%s: %s_%d%s.png is %s, its sprite page is %s" % (where, fam, p, suffix, Image.open(path).size, sz))
        md = os.path.join(META, fam)
        for g in sorted(os.listdir(md)) if os.path.isdir(md) else ():
            if g.endswith(".yaml") and g[:-5] not in a["sprites"]:
                err("%s/%s: has metadata but is not in the atlas (run atlasgen)" % (where, g[:-5]))
    return atlases


def pawn_keys(atlas):
    """Every pawn sprite key: individual files plus the atlas."""
    d = os.path.join(GEN, "pawn")
    keys = {f[:-4] for f in os.listdir(d)
            if f.endswith(".png") and not f.endswith("_mask.png") and not f.endswith("_n.png")} if os.path.isdir(d) else set()
    return keys | set(atlas)


def build_key(layer, build, facing):
    """<layer>_<build>_<facing>; m_avg keeps the pre-NG-1 un-suffixed name."""
    tag = layer if build == LEGACY_BUILD else "%s_%s" % (layer, build)
    return "%s_%s" % (tag, facing)


def pawn_expected():
    """{key: facing} of the whole pawn contract (no hands: the renderer draws none)."""
    out = {}
    for f in PAWN_FACINGS:
        out.update({build_key(l, b, f): f for b in PAWN_BUILDS for l in BUILD_LAYERS})
        out.update({"%s_%s" % (l, f): f for l in SHARED_LAYERS})
    return out


def _pawn_meta(key, atlas):
    """(canvas, {attachment: [x, y]}) of a pawn sprite from its sidecar, else the atlas."""
    p = os.path.join(META, "pawn", key + ".yaml")
    if os.path.exists(p):
        text = open(p, encoding="utf-8").read()
        att = {m.group(1): [float(m.group(2)), float(m.group(3))] for m in
               re.finditer(r"^\s{2}(\w+):\s*\[([-\d.]+),\s*([-\d.]+)\]", text.split("attachments:", 1)[1], re.M)} \
            if "\nattachments:" in text else {}
        return tuple(ints(field(text, "canvas"))), att
    s = atlas.get(key)
    return (tuple(s["canvas"]) if s and "canvas" in s else None), (s["attach"] if s else {})


def check_pawn_layers(atlas=None):
    """The pawn contract (ART §10): exactly one standing sprite per build x per-build
    layer x facing (south, east, north) and per shared layer x facing; no frame
    suffixes, no west, no hand sprite; every layer of a facing on one canvas; bodies
    carry head/hold/back (not the removed hand/hand_l)."""
    atlas = (atlas or {}).get("pawn", {})
    have = pawn_keys(atlas)
    want = pawn_expected()
    for key in sorted(set(want) - have):
        err("pawn/%s: missing" % key)
    extra = sorted(have - set(want))
    if extra:
        err("pawn: %d sprites outside the contract (frame suffix, west facing or stale layer), e.g. %s"
            % (len(extra), ", ".join(extra[:5])))
    canvases = {}
    for key, facing in sorted(want.items()):
        if key not in have:
            continue
        canvas, att = _pawn_meta(key, atlas)
        if canvas:
            canvases.setdefault(facing, {}).setdefault(canvas, key)
        need = BODY_ATTACH if key.startswith("body_") else ()
        for a in need:
            if a not in att:
                err("pawn/%s: missing %s attachment" % (key, a))
            elif canvas and not (0 <= att[a][0] <= canvas[0] and 0 <= att[a][1] <= canvas[1]):
                err("pawn/%s: %s attachment %s outside canvas %s" % (key, a, att[a], canvas))
        for a in GONE_ATTACH:
            if a in att:
                err("pawn/%s: %s attachment was removed (use hold)" % (key, a))
    for facing, seen in sorted(canvases.items()):
        if len(seen) != 1:
            err("pawn/%s: layers disagree on canvas %s" % (facing, ", ".join(
                "%s (%s)" % (c, k) for c, k in sorted(seen.items()))))


def _roster_module():
    """The binding animal roster (ART-011), read from the Blender family."""
    sys.path.insert(0, os.path.join(ROOT, "source", "blender", "artlib", "families"))
    try:
        import animal_roster
    finally:
        sys.path.pop(0)
    return animal_roster


def _roster():
    return _roster_module().ROSTER


# PLAY-022 memory budget: animal + corpse PNG pixels (colour + normal maps)
# stay within ANIMAL_PX_CAP x the pre-PLAY-022 set (animal 281.2 M + corpse
# 10.1 M); size classes hit their area (animal_roster.SIZE_AREA) within
# AREA_TOL, measured on the east standing frame's alpha box at its ppt.
ANIMAL_PX_BASELINE = 291.3e6
ANIMAL_PX_CAP = 1.2
AREA_TOL = 0.15
AREA_ALPHA = 40


def _sprite_meta(path):
    """canvas, origin_px, ppt and gait of one metadata file (None if absent)."""
    if not os.path.exists(path):
        return None
    md = open(path, encoding="utf-8").read()
    ppt, gait = field(md, "ppt"), field(md, "gait")
    return {"canvas": tuple(ints(field(md, "canvas"))), "origin": tuple(ints(field(md, "origin_px"))),
            "ppt": int(ppt) if ppt and ppt.isdigit() else None, "gait": float(gait) if gait else None}


def _edge_alpha(path):
    """True when any pixel on the canvas border is not transparent: the
    tight canvas (C2) clipped the sprite or its outline."""
    a = Image.open(path).getchannel("A")
    w, h = a.size
    for box in ((0, 0, w, 1), (0, h - 1, w, h), (0, 0, 1, h), (w - 1, 0, w, h)):
        if a.crop(box).getextrema()[1] > 0:
            return True
    return False


def check_animal_density(roster, sizes):
    """PLAY-022: every animal and corpse sprite declares ppt; every animal
    file of a set declares the same gait; nothing touches the canvas
    border; each size class's median east standing area is within
    AREA_TOL of its target; and the pixel total stays within the budget."""
    ad, idir = os.path.join(GEN, "animal"), os.path.join(GEN, "item")
    for f in sorted(os.listdir(ad)) if os.path.isdir(ad) else ():
        if f.endswith(".png") and not f.endswith(("_n.png", "_mask.png")) and _edge_alpha(os.path.join(ad, f)):
            err("animal/%s: alpha on the canvas border (tight canvas clipped it)" % f[:-4])
    for sp in roster:
        for v in range(3):
            name = "corpse_%s_v%d" % (sp.key, v)
            m = _sprite_meta(os.path.join(META, "item", name + ".yaml"))
            if m is None:
                continue
            if m["ppt"] is None:
                err("item/%s: missing ppt" % name)
            png = os.path.join(idir, name + ".png")
            if os.path.exists(png) and _edge_alpha(png):
                err("item/%s: alpha on the canvas border (tight canvas clipped it)" % name)
    areas = {}
    for sp in roster:
        m = _sprite_meta(os.path.join(META, "animal", "%s_east.yaml" % sp.key))
        png = os.path.join(ad, "%s_east.png" % sp.key)
        if m is None or m["ppt"] is None or not os.path.exists(png):
            continue
        box = Image.open(png).getchannel("A").point(lambda v: 255 if v > AREA_ALPHA else 0).getbbox()
        if box:
            areas.setdefault(sp.size.rstrip("+-"), []).append(
                (box[2] - box[0]) * (box[3] - box[1]) / float(m["ppt"] ** 2))
    for cls, target in sizes.items():
        got = sorted(areas.get(cls, []))
        if not got:
            continue
        n = len(got)
        med = got[n // 2] if n % 2 else (got[n // 2 - 1] + got[n // 2]) / 2
        print("animal size %-2s median area %.2f tiles^2 (target %.1f, %+.0f%%, %d species)"
              % (cls, med, target, 100 * (med / target - 1), n))
        if abs(med / target - 1) > AREA_TOL:
            err("animal size %s: median east standing area %.2f tiles^2 is not within %d%% of %.1f"
                % (cls, med, AREA_TOL * 100, target))
    total = 0
    for d, pat in ((ad, r"\.png$"), (idir, r"^corpse_.*\.png$")):
        for f in os.listdir(d) if os.path.isdir(d) else ():
            if re.search(pat, f) and not f.endswith("_mask.png"):
                w, h = Image.open(os.path.join(d, f)).size
                total += w * h
    print("animal + corpse pixels %.1f M (%.2fx the %.1f M baseline, cap %.1fx)"
          % (total / 1e6, total / ANIMAL_PX_BASELINE, ANIMAL_PX_BASELINE / 1e6, ANIMAL_PX_CAP))
    if total > ANIMAL_PX_CAP * ANIMAL_PX_BASELINE:
        err("animal + corpse pixels %.1f M exceed %.1fx the %.1f M baseline"
            % (total / 1e6, ANIMAL_PX_CAP, ANIMAL_PX_BASELINE / 1e6))


def check_animals():
    """Every roster species has its full set (ART §11.1, §27.1-27.2): per
    facing the standing frame, trot walk01..08, its attack set 1..4,
    graze1..2 and flinch1..2 on the standing frame's canvas; corpses
    v0..v2; and the same animal set under <key>_f for dimorphic species.
    Facings are south/east/north only (the renderer mirrors east for west),
    so a west sprite is a stale error. Sprites of species outside the
    roster are errors."""
    ad = os.path.join(GEN, "animal")
    mod = _roster_module()
    roster = mod.ROSTER
    bases = set()
    for sp in roster:
        frames = TROT_FRAMES + _set(sp.attack, 4) + ANIMAL_COMMON
        for base in [sp.key] + ([sp.key + "_f"] if sp.dimorphic else []):
            bases.add(base)
            gaits = set()
            for facing in ANIMAL_FACINGS:
                stand = os.path.join(ad, "%s_%s.png" % (base, facing))
                if not os.path.exists(stand):
                    err("animal/%s_%s: missing standing frame" % (base, facing))
                    continue
                size = Image.open(stand).size
                sm = _sprite_meta(os.path.join(META, "animal", "%s_%s.yaml" % (base, facing)))
                if sm is not None:
                    # C2: one canvas, origin and ppt per species, sex and facing;
                    # east is symmetric about the tile (the renderer mirrors it for west)
                    if sm["ppt"] is None:
                        err("animal/%s_%s: missing ppt" % (base, facing))
                    elif facing == "east" and size[0] != 2 * sm["origin"][0] + sm["ppt"]:
                        err("animal/%s_%s: canvas not symmetric about the tile (west mirrors east)" % (base, facing))
                    if not sm["gait"] or sm["gait"] <= 0:
                        err("animal/%s_%s: missing gait (tiles per walk cycle)" % (base, facing))
                    gaits.add(sm["gait"])
                for fr in frames:
                    p = os.path.join(ad, "%s_%s_%s.png" % (base, facing, fr))
                    if not os.path.exists(p):
                        err("animal/%s_%s: missing frame %s" % (base, facing, fr))
                        continue
                    if Image.open(p).size != size:
                        err("animal/%s_%s_%s: canvas differs from the standing frame" % (base, facing, fr))
                    fm = _sprite_meta(os.path.join(META, "animal", "%s_%s_%s.yaml" % (base, facing, fr)))
                    if sm is not None and fm is not None:
                        for k in ("origin", "ppt", "gait"):
                            if fm[k] != sm[k]:
                                err("animal/%s_%s_%s: %s %s differs from the standing frame's %s"
                                    % (base, facing, fr, k, fm[k], sm[k]))
            if len(gaits) > 1:
                err("animal/%s: gait differs between facings %s" % (base, sorted(g for g in gaits if g)))
        for v in range(3):
            if not os.path.exists(os.path.join(GEN, "item", "corpse_%s_v%d.png" % (sp.key, v))):
                err("item/corpse_%s_v%d: missing corpse" % (sp.key, v))
    keys = {sp.key for sp in roster}
    for f in sorted(os.listdir(ad)) if os.path.isdir(ad) else ():
        if re.match(r"^\w+?_west(_\w+)?\.png$", f):
            err("animal/%s: west is east mirrored by the renderer; stale sprite" % f[:-4])
            continue
        m = re.match(r"^(\w+?)_(south|east|north)\.png$", f)
        if m and m.group(1) not in bases:
            err("animal/%s: species not in the roster" % f[:-4])
    for f in sorted(os.listdir(os.path.join(GEN, "item"))):
        m = re.match(r"^corpse_(\w+?)(?:_v\d)?\.png$", f)
        if m and not f.endswith("_n.png") and m.group(1) not in keys:
            err("item/%s: corpse of a species not in the roster" % f[:-4])
    check_animal_density(roster, mod.SIZE_AREA)


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
        if not os.path.isdir(os.path.join(META, family)):
            continue  # e.g. orphan_allow.txt (asset housekeeping list)
        for f in os.listdir(os.path.join(META, family)):
            img = re.search(r"^image: (\S+)", open(os.path.join(META, family, f)).read(), re.M)
            if img and not os.path.exists(os.path.join(ROOT, img.group(1))):
                err("%s/%s: metadata points at missing image %s" % (family, f, img.group(1)))
    atlases = check_atlas()
    check_terrain_seams()
    check_autotile()
    check_pawn_layers(atlases)
    check_animals()
    check_gear()
    for e in errors:
        print("ERROR", e)
    print("%d sprites checked, %d errors" % (count, len(errors)))
    sys.exit(1 if errors else 0)


if __name__ == "__main__":
    main()
