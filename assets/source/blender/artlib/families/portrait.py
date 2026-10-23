"""Storyteller busts (ART spec §17.1, NG-1 §7.2): three original characters
rendered through the canonical stage and light as UI art.

Orla Vane (steady), Bram Holloway (hearth), Kestrel Quay (gambler). Each is
a 512x640 transparent bust plus a 120x150 thumbnail:

    generated/ui/portraits/<key>.png, <key>_thumb.png
    metadata/ui_portrait/<key>.yaml, <key>_thumb.yaml  (catalog key ui_portrait/<key>)

The bust is modelled FRONT-ON, lying flat in the stage's ground plane: screen
up is +Y, the face points at the camera (+Z), and depth is Z. Because the
canonical shear lifts a point by SHEAR_K * z, every part is placed with
`_at(u, v, z)`, which pre-compensates so (u, v) lands exactly where the part
should appear on the 4 x 5 tile canvas (u right, v up from the bottom).
Faces then sit in the "base" toon band, curved forms catch light from the
upper left like the rest of the art. The 3/4 turn is faked with the head
shifted and turned within the bust, a pose cheaper than a second rig.

Materials are fixed colours (palettes/materials_ng1.yaml), never tinted.

    make art FAMILIES=portrait
"""
import math
import os
from contextlib import contextmanager

import numpy as np
from mathutils import Matrix, Vector

from .. import contracts as C
from .. import meta, palette, post
from .. import stage as S

W_TILES, H_TILES = 4, 5          # 512 x 640 px
THUMB = (120, 150)
UI_DIR = os.path.join(C.ASSETS_DIR, "generated", "ui", "portraits")


# ---------------------------------------------------------------- helpers
def _at(u, v, z):
    """Stage coordinates that appear at canvas point (u, v) with depth z."""
    return (u, v - H_TILES - C.SHEAR_K * z, z)


def ell(name, u, v, z, ru, rv, rz, mat, segs=24, rings=14):
    cx, cy, cz = _at(u, v, z)
    return S.ellipsoid(name, cx, cy, cz, ru, rv, rz, mat, segs=segs, rings=rings)


def ell_rot(name, u, v, z, ru, rv, rz, mat, deg, segs=16, rings=8, flat=None):
    """An ellipsoid turned `deg` (counter-clockwise on the canvas) about its centre."""
    ob = ell(name, u, v, z, ru, rv, rz, mat, segs=segs, rings=rings)
    c = Vector(_at(u, v, z))
    ob.matrix_world = Matrix.Translation(c) @ Matrix.Rotation(math.radians(deg), 4, "Z") @ Matrix.Translation(-c)
    if flat:
        _paint(ob, flat)
    return ob


def rbox(name, u0, v0, u1, v1, z0, z1, mat, bevel=0.05, segments=2):
    """Box spanning canvas (u0..u1, v0..v1), depth z0..z1."""
    x0, y0, a = _at(u0, v0, z0)
    x1, y1, b = _at(u1, v1, z0)
    return S.box(name, x0, y0 - C.SHEAR_K * (z1 - z0) / 2, z0, x1, y1 - C.SHEAR_K * (z1 - z0) / 2, z1, mat,
                 bevel=bevel, segments=segments)


def head_z(h, du, dv):
    """Depth of a head ellipsoid's front surface at offset (du, dv) from its centre."""
    t = 1.0 - (du / h["ru"]) ** 2 - (dv / h["rv"]) ** 2
    return h["z"] + h["rz"] * math.sqrt(max(t, 0.0))


def _paint(ob, hex_):
    """Give an object one flat (unshaded) colour: crisp line work for the face."""
    ob.data.materials.clear()
    ob.data.materials.append(S.flat("pt_" + hex_.lstrip("#"), palette.hex_to_linear(hex_)))


def tube(name, pts, r, mat, caps=True, segs=10, flat=None):
    """A round tube through canvas points [(u, v, z), ...]; `flat` (hex) paints it unshaded."""
    n0 = len(S.objects())
    ws = [Vector(_at(*p)) for p in pts]
    for i, (a, b) in enumerate(zip(ws, ws[1:])):
        d = b - a
        ln = d.length
        if ln < 1e-5:
            continue
        S.cyl("%s_%d" % (name, i), 0, 0, r, 0.0, ln, mat, segs=segs, bevel=0.0)
        ob = S.objects()[-1]
        ob.matrix_world = Matrix.Translation(a) @ Vector((0, 0, 1)).rotation_difference(d.normalized()).to_matrix().to_4x4()
    if caps:
        for i, w in enumerate(ws):
            S.ellipsoid("%s_j%d" % (name, i), w.x, w.y, w.z, r, r, r, mat, segs=segs, rings=6)
    if flat:
        for ob in S.objects()[n0:]:
            _paint(ob, flat)


def disc(name, u, v, z, r, depth, mat, segs=14):
    """A round button / coin facing the viewer."""
    cx, cy, cz = _at(u, v, z)
    S.cyl(name, cx, cy - C.SHEAR_K * depth / 2, r, cz, cz + depth, mat, segs=segs, bevel=min(0.01, depth / 3))


# ------------------------------------------------------------- shared head
def _head(skin, h, ears=True, cheeks=0.0):
    """Head, neck and ears; returns the head parameters for placing features."""
    ell("head", h["u"], h["v"], h["z"], h["ru"], h["rv"], h["rz"], skin, segs=40, rings=26)
    ell("neck", h["u"], h["v"] - h["rv"] * 0.9, 0.28, 0.40, 0.62, 0.30, skin)
    if ears:
        for sx in (-1, 1):
            ell("ear%d" % sx, h["u"] + sx * (h["ru"] * 0.98), h["v"] - 0.02, h["z"] - 0.12, 0.13, 0.22, 0.12, skin, segs=14, rings=8)
    if cheeks:
        for sx in (-1, 1):
            ell("cheek%d" % sx, h["u"] + sx * h["ru"] * 0.52, h["v"] - h["rv"] * 0.32, h["z"] + 0.1, 0.30, 0.26, cheeks, skin,
                segs=18, rings=10)
    return h


def _eye(h, sx, dv=0.10, du=0.30, ru=0.115, rv=0.075, look=0.0, lid=0.0):
    """One eye on the face: white, dark iris, a catchlight and an upper lid."""
    u = h["u"] + sx * du
    v = h["v"] + dv
    z = head_z(h, sx * du, dv)
    ell("eyew", u, v, z - 0.03, ru, rv, 0.05, "eye_white", segs=16, rings=8)
    ell("iris", u + look * 0.03, v - 0.005, z + 0.01, ru * 0.55, rv * 0.95, 0.04, "hair_ink", segs=12, rings=6)
    ell("glint", u + look * 0.03 - 0.025, v + 0.025, z + 0.045, 0.018, 0.018, 0.012, "eye_white", segs=6, rings=4)
    if lid:
        ell("lid", u, v + rv * 0.55, z + 0.0, ru * 1.1, rv * 0.62 + lid, 0.05, h["skin"], segs=14, rings=6)


def _brow(h, sx, du=0.31, dv=0.27, w=0.2, ang=6.0, mat="hair_silver", thick=0.05, arch=0.0):
    """An eyebrow: a flat ellipsoid; ang raises the outer end (degrees)."""
    u = h["u"] + sx * du
    v = h["v"] + dv + arch
    ell_rot("brow", u, v, head_z(h, u - h["u"], v - h["v"]) + 0.0, w / 2, thick, 0.05, mat, sx * ang, segs=14, rings=8)


def _nose(h, skin, shade, dv=-0.06, size=1.0):
    z = head_z(h, 0, dv)
    ell("nose", h["u"], h["v"] + dv, z - 0.02, 0.095 * size, 0.15 * size, 0.1 * size, skin, segs=14, rings=8)
    for sx in (-1, 1):
        ell("nostril", h["u"] + sx * 0.07 * size, h["v"] + dv - 0.13 * size, z + 0.0, 0.035 * size, 0.022 * size, 0.03, shade,
            segs=8, rings=5)


def _mouth(h, dv=-0.45, w=0.26, smile=0.05, skew=0.0, flat="#a8554b", thick=0.024):
    """A line of five points: the ends lift for a smile, `skew` lifts the right corner more."""
    pts = []
    for i in range(5):
        t = i / 4.0 * 2 - 1                                  # -1 .. 1
        uu = h["u"] + t * w
        vv = h["v"] + dv + smile * t * t + skew * max(t, 0.0) ** 2
        pts.append((uu, vv, head_z(h, uu - h["u"], vv - h["v"]) + 0.012))
    tube("mouth", pts, thick, "lip", segs=8, flat=flat)


# ----------------------------------------------------------------- Orla Vane
def _orla():
    """Orla Vane, the Steady Hand: late 50s, close-cropped silver hair, calm
    half-smile, a dispatcher's headset round her neck, quilted navy jacket with
    a reflective stripe."""
    skin, shade = "skin_fair", "skin_ruddy"
    h = dict(u=2.0, v=3.42, z=0.62, ru=0.80, rv=0.98, rz=0.85, skin=skin)
    # torso: quilted jacket (pads stacked with seams), shoulders, collar
    ell("shoulder_l", 0.92, 1.62, 0.20, 0.86, 0.66, 0.34, "jacket_navy", segs=26, rings=14)
    ell("shoulder_r", 3.08, 1.62, 0.20, 0.86, 0.66, 0.34, "jacket_navy", segs=26, rings=14)
    rbox("chest", 0.55, -0.4, 3.45, 2.12, 0.0, 0.55, "jacket_navy", bevel=0.22, segments=3)
    _head(skin, h, cheeks=0.22)
    for k, v in enumerate((0.15, 0.70, 1.25)):                   # quilting pads
        rbox("pad%d" % k, 0.62, v, 3.38, v + 0.52, 0.5, 0.62, "jacket_navy", bevel=0.07)
    for sx in (-1, 1):                                         # sleeve pads
        for k, v in enumerate((0.45, 1.05)):
            rbox("spad", 2.0 + sx * 1.56 - 0.28, v, 2.0 + sx * 1.56 + 0.28, v + 0.5, 0.40, 0.55, "jacket_navy", bevel=0.07)
    # reflective stripe across the chest and down both sleeves
    rbox("stripe", 0.62, 0.62, 3.38, 0.82, 0.60, 0.66, "reflect", bevel=0.02, segments=1)
    for sx in (-1, 1):
        rbox("sstripe", 2.0 + sx * 1.56 - 0.3, 0.9, 2.0 + sx * 1.56 + 0.3, 1.04, 0.5, 0.57, "reflect", bevel=0.01, segments=1)
    rbox("zip", 1.97, 0.1, 2.03, 2.05, 0.58, 0.64, "steel", bevel=0.01, segments=1)
    ell("collar", 2.0, 2.18, 0.38, 0.78, 0.30, 0.30, "jacket_navy", segs=28, rings=10)
    ell("collar_in", 2.0, 2.20, 0.42, 0.50, 0.2, 0.24, "skin_fair", segs=20, rings=8)
    # dispatcher's headset resting round the neck: cups on the collarbones, band behind, boom mic
    for sx in (-1, 1):
        cu = 2.0 + sx * 1.02
        ell("cup%d" % sx, cu, 1.86, 0.78, 0.26, 0.32, 0.17, "steel_dark", segs=18, rings=10)
        ell("cup_pad%d" % sx, cu - sx * 0.01, 1.86, 0.72, 0.30, 0.36, 0.14, "metal_dark", segs=18, rings=10)
        tube("band%d" % sx, [(cu, 2.12, 0.74), (2.0 + sx * 0.93, 2.36, 0.62), (2.0 + sx * 0.82, 2.56, 0.5)], 0.055, "steel_dark")
    tube("boom", [(2.0 - 1.02 + 0.05, 1.62, 0.8), (1.45, 1.38, 0.85), (1.78, 1.28, 0.9)], 0.04, "steel_dark", segs=8)
    ell("mic", 1.84, 1.27, 0.92, 0.075, 0.075, 0.07, "metal_dark", segs=12, rings=8)
    # face
    for sx in (-1, 1):
        _eye(h, sx, ru=0.13, rv=0.085, look=0.4, lid=0.02)
        _brow(h, sx, du=0.31, dv=0.27, w=0.24, ang=8.0, mat="hair_silver", thick=0.045)
    _nose(h, skin, shade, size=0.95)
    _mouth(h, dv=-0.46, w=0.25, smile=0.03, skew=0.05)
    for sx in (-1, 1):   # laugh lines
        for i in range(2):
            u = h["u"] + sx * (0.40 + i * 0.04)
            v = h["v"] - 0.36 + i * 0.10
            u2, v2 = u + sx * 0.03, v - 0.08
            tube("laugh", [(u, v, head_z(h, u - h["u"], v - h["v"]) + 0.01), (u2, v2, head_z(h, u2 - h["u"], v2 - h["v"]) + 0.01)],
                 0.012, shade, segs=6, flat="#cf9b84")
    # close-cropped silver hair: a cap pushed back so the face breaks through it, plus short sideburns
    hair = dict(u=h["u"], v=h["v"] + 0.30, z=0.50, ru=0.87, rv=0.92, rz=0.88)
    ell("hair", hair["u"], hair["v"], hair["z"], hair["ru"], hair["rv"], hair["rz"], "hair_silver", segs=40, rings=26)
    for sx in (-1, 1):
        ell("burn%d" % sx, h["u"] + sx * 0.76, h["v"] + 0.12, 0.62, 0.10, 0.28, 0.14, "hair_silver", segs=12, rings=8)
    rng = np.random.RandomState(11)
    for i in range(26):                       # cropped texture: small tufts over the dome
        a_ = rng.uniform(0.2, math.pi - 0.2)
        r = rng.uniform(0.25, 0.72)
        du, dv = math.cos(a_) * r * 0.9, 0.30 + math.sin(a_) * r * 0.85
        if dv < 0.45:
            continue
        z = head_z(hair, du, dv - 0.30)
        ell("tuft", h["u"] + du, h["v"] + dv, z - 0.02, 0.10, 0.07, 0.07, "hair_silver", segs=8, rings=5)
    return None


# ------------------------------------------------------------- Bram Holloway
def _bram():
    """Bram Holloway, the Hearthkeeper: 60s, broad and ruddy, short white beard,
    knitted cardigan over a canvas apron, a steaming mug held in both hands."""
    skin, shade = "skin_ruddy", "skin_umber"
    h = dict(u=2.0, v=3.40, z=0.64, ru=0.92, rv=0.97, rz=0.85, skin=skin)
    # broad shoulders in a knitted cardigan; open front shows the apron
    ell("shoulder_l", 0.76, 1.62, 0.20, 0.90, 0.68, 0.34, "knit_rust", segs=26, rings=14)
    ell("shoulder_r", 3.24, 1.62, 0.20, 0.90, 0.68, 0.34, "knit_rust", segs=26, rings=14)
    rbox("chest", 0.30, -0.4, 3.70, 2.12, 0.0, 0.52, "knit_rust", bevel=0.24, segments=3)
    _head(skin, h, cheeks=0.3)
    rbox("shirt", 1.32, -0.2, 2.68, 2.05, 0.46, 0.58, "knit_cream", bevel=0.09)
    rbox("apron", 1.38, -0.2, 2.62, 1.72, 0.58, 0.70, "apron_canvas", bevel=0.07)
    rbox("pocket", 1.58, 0.15, 2.42, 0.78, 0.69, 0.76, "apron_canvas", bevel=0.05)
    rbox("pocket_seam", 1.58, 0.74, 2.42, 0.80, 0.75, 0.78, "knit_cream", bevel=0.0, segments=1)
    for sx in (-1, 1):                                          # apron straps to the neck
        tube("strap%d" % sx, [(2.0 + sx * 0.55, 1.70, 0.74), (2.0 + sx * 0.52, 2.0, 0.66), (2.0 + sx * 0.36, 2.32, 0.58)], 0.07, "apron_canvas")
    for sx in (-1, 1):                                          # cardigan fronts: raised ribs, buttonband, buttons
        x0 = 1.38 if sx < 0 else 2.62
        far = 0.40 if sx < 0 else 3.60
        for i in range(7):
            u = (far + (x0 - far) * (i + 0.5) / 7.0)
            rbox("rib", u - 0.035, -0.1, u + 0.035, 2.0, 0.50, 0.60, "knit_rust", bevel=0.02, segments=1)
        rbox("band%d" % sx, x0 - 0.12 if sx < 0 else x0, -0.1, x0 if sx < 0 else x0 + 0.12, 2.0, 0.52, 0.66, "knit_rust", bevel=0.04)
    for v in (0.2, 0.75, 1.3):
        disc("btn", 1.28, v, 0.62, 0.075, 0.05, "knit_cream", segs=12)
    ell("collar", 2.0, 2.02, 0.36, 0.92, 0.26, 0.28, "knit_rust", segs=28, rings=10)
    # face
    for sx in (-1, 1):
        _eye(h, sx, dv=0.08, du=0.34, ru=0.115, rv=0.07, look=-0.4, lid=0.035)
        _brow(h, sx, du=0.36, dv=0.28, w=0.34, ang=-6.0, mat="hair_white", thick=0.075)
        ell("pouch%d" % sx, h["u"] + sx * 0.34, h["v"] - 0.06, head_z(h, sx * 0.34, -0.06) - 0.01, 0.16, 0.05, 0.04, shade,
            segs=12, rings=6)
    _nose(h, "skin_ruddy", "skin_umber", dv=-0.04, size=1.35)
    ell("nose_tip", h["u"], h["v"] - 0.14, head_z(h, 0, -0.14) + 0.02, 0.15, 0.1, 0.09, "skin_ruddy", segs=14, rings=8)
    # short white beard hugging the jaw, a moustache over the smile, white tufts at the temples
    beard = dict(u=h["u"], v=h["v"] - 0.30, z=0.56, ru=0.99, rv=0.88, rz=0.85)
    ell("beard", beard["u"], beard["v"], beard["z"], beard["ru"], beard["rv"], beard["rz"], "hair_white", segs=40, rings=24)
    def surf(du, dv):        # the outer surface of face or beard at a canvas offset from the head centre
        return max(head_z(h, du, dv), head_z(beard, du, dv + 0.30))

    for sx in (-1, 1):
        ell("stache%d" % sx, h["u"] + sx * 0.17, h["v"] - 0.29, surf(sx * 0.17, -0.29) + 0.01, 0.22, 0.09, 0.09,
            "hair_white", segs=14, rings=8)
        ell("sidehair%d" % sx, h["u"] + sx * 0.84, h["v"] + 0.18, 0.5, 0.11, 0.30, 0.15, "hair_white", segs=12, rings=8)
        ell("tuft%d" % sx, h["u"] + sx * 0.62, h["v"] + 0.62, 0.55, 0.17, 0.12, 0.12, "hair_white", segs=12, rings=8)
    pts = []
    for i in range(5):
        t = i / 4.0 * 2 - 1
        uu = h["u"] + t * 0.18
        vv = h["v"] - 0.47 + 0.04 * t * t
        pts.append((uu, vv, surf(uu - h["u"], vv - h["v"]) + 0.02))
    tube("mouth", pts, 0.026, "lip", segs=8, flat="#8c4640")
    ell("brow_shine", h["u"] - 0.18, h["v"] + 0.72, head_z(h, -0.18, 0.72) + 0.0, 0.14, 0.05, 0.03, "skin_fair", segs=10, rings=6)
    # the mug, held up in both hands at chest height, steaming
    mu, mv = 2.92, 1.46
    cx, cy, cz = _at(mu, mv - 0.30, 0.98)
    S.cyl("mug", 0, 0, 0.32, 0.0, 0.62, "mug_blue", segs=24, bevel=0.012)
    S.objects()[-1].matrix_world = Matrix.Translation((cx, cy, cz)) @ Matrix.Rotation(math.radians(-90), 4, "X")
    S.cyl("coffee", 0, 0, 0.27, 0.60, 0.64, "knit_rust", segs=20, bevel=0.0)
    S.objects()[-1].matrix_world = Matrix.Translation((cx, cy, cz)) @ Matrix.Rotation(math.radians(-90), 4, "X")
    S.cyl("band", 0, 0, 0.325, 0.22, 0.30, "knit_cream", segs=24, bevel=0.0)
    S.objects()[-1].matrix_world = Matrix.Translation((cx, cy, cz)) @ Matrix.Rotation(math.radians(-90), 4, "X")
    ell("handle", mu + 0.40, mv, 0.98, 0.14, 0.2, 0.06, "mug_blue", segs=14, rings=8)
    ell("handle_hole", mu + 0.40, mv, 1.03, 0.066, 0.11, 0.05, "backdrop", segs=12, rings=6)
    for k, du in enumerate((-0.13, 0.0, 0.13)):               # fingers wrapped round the front, thumb on top
        ell("finger%d" % k, mu - 0.34 + du * 0.3, mv - 0.2 + 0.1 * k, 1.1, 0.085, 0.15, 0.09, skin, segs=12, rings=8)
    ell("thumb", mu + 0.12, mv + 0.22, 1.1, 0.10, 0.075, 0.08, skin, segs=12, rings=8)
    ell_rot("forearm", 3.16, 0.35, 0.72, 0.34, 1.05, 0.30, "knit_rust", -10, segs=22, rings=12)
    ell("cuff", 3.0, 1.02, 0.9, 0.31, 0.15, 0.2, "knit_cream", segs=16, rings=8)
    for i, (su, sv, sr) in enumerate(((2.92, 2.1, 0.08), (3.06, 2.32, 0.07), (2.88, 2.52, 0.06), (3.02, 2.72, 0.055))):
        ell("steam%d" % i, su, sv, 0.95, sr, sr * 1.2, 0.05, "med_white", segs=10, rings=6)
    return None


# ------------------------------------------------------------- Kestrel Quay
def _kestrel():
    """Kestrel Quay, the Gambler: 30s, angular, asymmetric dark hair with one
    shaved side, a long wine coat with brass buttons, a card held between two
    fingers, one raised eyebrow."""
    skin, shade = "skin_umber", "skin_umber"
    h = dict(u=1.92, v=3.40, z=0.62, ru=0.70, rv=1.0, rz=0.80, skin=skin)
    # narrower, sloped shoulders in a long coat with big lapels
    ell("shoulder_l", 0.98, 1.62, 0.20, 0.82, 0.62, 0.32, "coat_wine", segs=26, rings=14)
    ell("shoulder_r", 3.0, 1.60, 0.20, 0.82, 0.62, 0.32, "coat_wine", segs=26, rings=14)
    rbox("chest", 0.60, -0.4, 3.38, 2.05, 0.0, 0.50, "coat_wine", bevel=0.2, segments=3)
    _head(skin, h, ears=True)
    # angular jaw and chin, cheekbones
    ell("chin", h["u"], h["v"] - 0.80, 0.40, 0.38, 0.34, 0.34, skin, segs=20, rings=12)
    for sx in (-1, 1):
        ell("jaw%d" % sx, h["u"] + sx * 0.42, h["v"] - 0.62, 0.34, 0.20, 0.34, 0.30, skin, segs=14, rings=8)
        ell("cheekbone%d" % sx, h["u"] + sx * 0.46, h["v"] - 0.06, head_z(h, sx * 0.46, -0.06) - 0.06, 0.15, 0.12, 0.08, skin,
            segs=12, rings=8)
    # shirt and lapels, brass-buttoned double row
    rbox("shirt", 1.50, -0.2, 2.34, 2.1, 0.44, 0.56, "card_paper", bevel=0.08)
    rbox("vest", 1.62, -0.2, 2.22, 1.5, 0.54, 0.60, "hair_ink", bevel=0.05)
    for sx in (-1, 1):
        rbox("coat_front%d" % sx, 0.56 if sx < 0 else 2.28, -0.2, 1.56 if sx < 0 else 3.42, 2.0, 0.50, 0.62, "coat_wine_dk", bevel=0.08)
        pts = [(1.58, 2.18), (1.20, 1.14), (1.58, 0.1)] if sx < 0 else [(2.30, 2.18), (2.68, 1.14), (2.30, 0.1)]
        tube("lapel%d" % sx, [(p[0], p[1], 0.66) for p in pts], 0.075, "coat_lining", segs=8)
        for v in (0.18, 0.62, 1.06, 1.5):
            disc("btn%d" % sx, (1.12 if sx < 0 else 2.76) + (0.06 if v in (0.62, 1.5) else 0) * sx, v, 0.64, 0.085, 0.055, "brass", segs=14)
    ell("collar", h["u"], 2.2, 0.42, 0.74, 0.28, 0.30, "coat_wine", segs=28, rings=10)
    for sx in (-1, 1):
        ell_rot("collar_pt%d" % sx, h["u"] + sx * 0.50, 2.12, 0.62, 0.17, 0.48, 0.14, "coat_wine_dk", sx * -22, segs=16, rings=10)
    # face: sharp brows, one raised, a smirk
    hl, hr = -1, 1
    for sx in (-1, 1):
        _eye(h, sx, dv=0.06, du=0.27, ru=0.125, rv=0.08, look=0.8, lid=0.03)
    _brow(h, hl, du=0.27, dv=0.19, w=0.25, ang=-9.0, mat="hair_ink", thick=0.05)                         # flat, lowered
    _brow(h, hr, du=0.27, dv=0.36, w=0.27, ang=22.0, mat="hair_ink", thick=0.05, arch=0.03)             # raised
    _nose(h, skin, shade, dv=-0.10, size=0.9)
    _mouth(h, dv=-0.50, w=0.22, smile=0.015, skew=0.10, flat="#7a3b36", thick=0.024)
    # asymmetric hair: long swept lock on the left, shaved stubble on the right
    ell("hair_back", h["u"] - 0.14, h["v"] + 0.30, 0.50, 0.66, 0.92, 0.88, "hair_ink", segs=36, rings=24)
    ell("stubble", h["u"] + 0.30, h["v"] + 0.30, 0.50, 0.52, 0.90, 0.88, "stubble", segs=36, rings=24)
    ell("quiff", h["u"] - 0.26, h["v"] + 0.92, 0.80, 0.54, 0.24, 0.30, "hair_ink", segs=24, rings=12)
    for i, (du, dv, ru, rv) in enumerate(((-0.40, 0.50, 0.34, 0.32), (-0.60, 0.28, 0.26, 0.34), (-0.74, 0.02, 0.2, 0.34),
                                         (-0.78, -0.24, 0.14, 0.28))):
        ell("lock%d" % i, h["u"] + du, h["v"] + dv, head_z(h, du, dv) * 0.8 + 0.2, ru, rv, 0.22, "hair_ink", segs=16, rings=10)
    ell("fringe", h["u"] - 0.22, h["v"] + 0.52, head_z(h, -0.22, 0.52) + 0.03, 0.40, 0.17, 0.12, "hair_ink", segs=18, rings=8)
    # raised hand: two fingers pinching a card (a red diamond on white)
    hu, hv = 3.18, 2.42
    ell("palm", hu, hv - 0.22, 0.92, 0.22, 0.2, 0.12, skin, segs=14, rings=8)
    ell("thumb", hu - 0.16, hv - 0.14, 0.99, 0.07, 0.14, 0.08, skin, segs=10, rings=6)
    for k, (du, ang) in enumerate(((0.02, 4), (0.15, -6))):
        a = math.radians(ang)
        tube("finger%d" % k, [(hu + du, hv - 0.15, 0.98), (hu + du + 0.04 * math.sin(a), hv + 0.2, 1.0),
                              (hu + du + 0.1 * math.sin(a), hv + 0.50, 1.02)], 0.066, skin, segs=10)
    n0 = len(S.objects())
    rbox("card", hu - 0.20, hv + 0.34, hu + 0.34, hv + 1.14, 0.98, 1.02, "card_paper", bevel=0.02, segments=1)
    cx, cy, cz = _at(hu + 0.07, hv + 0.74, 1.03)
    d = S.box("pip", cx - 0.1, cy - 0.12, cz, cx + 0.1, cy + 0.12, cz + 0.02, "card_ink", bevel=0.0)
    d.matrix_world = Matrix.Translation((cx, cy, cz)) @ Matrix.Rotation(math.radians(45), 4, "Z") @ Matrix.Translation((-cx, -cy, -cz))
    for ob in S.objects()[n0:]:
        ob.matrix_world = Matrix.Translation((hu + 0.07, hv + 0.74 - H_TILES, 0)) @ Matrix.Rotation(math.radians(-14), 4, "Z") @ Matrix.Translation(
            (-(hu + 0.07), -(hv + 0.74 - H_TILES), 0)) @ ob.matrix_world
    tube("sleeve", [(3.40, -0.1, 0.5), (3.30, 0.8, 0.7), (3.22, 1.8, 0.86)], 0.32, "coat_wine", segs=16)
    rbox("cuff", 2.94, 2.0, 3.48, 2.18, 0.88, 0.98, "coat_lining", bevel=0.05)
    return None


# ------------------------------------------------------------------ render
CHARACTERS = {
    "steady": ("Orla Vane", _orla),
    "hearth": ("Bram Holloway", _bram),
    "gambler": ("Kestrel Quay", _kestrel),
}


@contextmanager
def _ui_output():
    """render_asset writes under generated/world/<family>; redirect the PNG to
    generated/ui/portraits and the sidecar to metadata/ui_portrait for this call."""
    real_out, real_write = S.out_path, meta.write

    def out_path(family, name):
        return os.path.join(UI_DIR, name + ".png")

    def write(family, name, data):
        data = dict(data, image="generated/ui/portraits/%s.png" % name)
        return real_write("ui_portrait", name, data)

    S.out_path, meta.write = out_path, write
    try:
        yield
    finally:
        S.out_path, meta.write = real_out, real_write


def _area_resize(img, w, h):
    """Box-filter downscale of straight-alpha RGBA (premultiplied internally)."""
    H, W = img.shape[:2]

    def weights(n_out, n_in):
        m = np.zeros((n_out, n_in), np.float32)
        scale = n_in / n_out
        for i in range(n_out):
            a, b = i * scale, (i + 1) * scale
            for j in range(int(a), min(int(math.ceil(b)), n_in)):
                m[i, j] = min(b, j + 1) - max(a, j)
        return m / m.sum(axis=1, keepdims=True)

    wy, wx = weights(h, H), weights(w, W)
    pm = img.copy()
    pm[..., :3] *= pm[..., 3:4]
    out = np.stack([wy @ pm[..., c] @ wx.T for c in range(4)], axis=-1)
    a = out[..., 3:4]
    out[..., :3] = np.where(a > 1e-5, out[..., :3] / np.maximum(a, 1e-5), 0.0)
    return np.clip(out, 0, 1)


def _portrait(key):
    name, builder = CHARACTERS[key]
    builder()
    with _ui_output():
        path = S.render_asset("portrait", key, footprint=(W_TILES, H_TILES), zmax=0.0, margins=(0, 0, 0, 0),
                              outline=False, shadow=False, tint=False,
                              extra={"portrait": key, "character": name, "thumbnail": key + "_thumb"})
    raw = S._read_png(path)
    full = post.outline(raw, width=5)
    S.write_png(full, path)
    thumb = post.outline(_area_resize(raw, *THUMB), width=2)
    tpath = os.path.join(UI_DIR, key + "_thumb.png")
    S.write_png(thumb, tpath)
    meta.write("ui_portrait", key + "_thumb",
               {"image": "generated/ui/portraits/%s_thumb.png" % key, "scale": 1, "canvas": list(THUMB),
                "footprint": [0, 0], "origin_px": [0, 0], "portrait": key, "character": name, "thumbnail": True,
                "source": "generated/ui/portraits/%s.png" % key,
                "shadow": {"kind": "none", "w": 0.0, "h": 0.0, "d": 0.0}})
    return [path, tpath]


def build():
    out = []
    for key in CHARACTERS:
        out += _portrait(key)
    return out
