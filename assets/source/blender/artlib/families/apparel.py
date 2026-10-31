"""Layered core clothing (TEMP-4, ART spec §10, §15): worn layers, ground
items and ui icons for the vanilla-core garments. All shapes are original.

Worn layers use the pawn rig (`pawn.py`): the same limbless capsule, six
builds, S/E/N facings (west is east mirrored), each rendered over a body
holdout so any subset composites. Keys follow the pawn family:
    pawn/<layer>_<build>_<facing>   body garments, every build (m_avg unsuffixed)
    pawn/<hat>_<facing>             hats, shared by every build, like tuque_knit
The stuff-coloured fabric is the neutral `cloth` group, so every layer has
a `_mask` and the renderer multiplies the stuff colour over it; buttons,
belts, cords and bands keep fixed colours.

Layers and draw order (shells grow over the body by G + the offset shown):
    skin    button_down (+0.004, like shirt_basic), tribalwear (+0.004,
            torso and legs in one piece, worn instead of pants + shirt)
    middle  jacket (+0.008, hem 0.14): over any shirt and the tribalwear,
            and inside the parka (+0.020 from 0.10) and duster (+0.012,
            flared), so an outer layer hides it wholly but for the collar
            the shirts also show
    head    cowboy_hat, bowler_hat (over hair, like the tuque)
The t-shirt and pants reuse shirt_basic and pants_basic (no new layers).

Ground items are item/<name> and ui icons ui_icon/<name> (cut from the
item like overlay.py's ICONS) for tshirt, button_down, pants, jacket,
tribalwear, cowboy_hat and bowler_hat.
    make art FAMILIES=apparel     (RWG_APPAREL=jacket,bowler_hat and
                                   RWG_PAWN_BUILDS=m_avg narrow a review build)
"""
import math
import os

import bmesh
import numpy as np

from .. import contracts as C
from .. import stage as S
from . import pawn as P
from .pawn import CX, CY, G, HEAD_Z, TOP, _extent, _shell


def _front(z, g):
    """Front surface y of the shell grown by g at height z."""
    return CY - _extent(z, g)[1]


# ------------------------------------------------------------ worn layers
def _button_down():
    """A collared shirt to below the belt: a raised placket with buttons,
    pointed collar wings and a chest pocket."""
    g = G + 0.004
    _shell("bshirt", 0.20, TOP, g, "cloth", round_hi=True)
    S.cyl("bcollar", CX, CY, 0.10, TOP - 0.02, TOP + 0.03, "cloth", segs=16, ry=0.085)
    y = _front(0.38, g)
    S.box("placket", CX - 0.022, y - 0.006, 0.20, CX + 0.022, y + 0.01, TOP - 0.03, "cloth", bevel=0.004)
    for z in (0.26, 0.34, 0.42, 0.50):
        yb = _front(z, g)
        S.ellipsoid("button", CX, yb - 0.005, z, 0.012, 0.005, 0.012, "ceramic", segs=8, rings=4)
    for sx in (-1, 1):
        S.prism("wing", [(CX + sx * 0.012, CY - 0.10), (CX + sx * 0.085, CY - 0.085), (CX + sx * 0.03, CY - 0.13)],
                TOP - 0.02, TOP + 0.03, "cloth", bevel=0.006)
    wx, _, _ = _extent(0.44, g)
    x = CX + 0.5 * wx
    yp = _front(0.44, g)
    S.box("pocket", x - 0.035, yp - 0.006, 0.39, x + 0.035, yp + 0.01, 0.47, "cloth", bevel=0.006)


def _jacket():
    """A hip-length jacket: a hem band, a front zip, two flap pockets and a
    low stand collar. Thin enough to stay inside the parka and duster."""
    g = G + 0.008
    _shell("jacket", 0.14, TOP, g, "cloth", round_hi=True)
    _shell("jhem", 0.14, 0.18, g + 0.003, "cloth")
    S.cyl("jcollar", CX, CY, 0.105, TOP - 0.03, TOP + 0.03, "cloth", segs=18, ry=0.09, bevel=0.006)
    y = _front(0.34, g)
    S.box("zip", CX - 0.008, y - 0.005, 0.15, CX + 0.008, y + 0.01, TOP - 0.03, "metal_dark", bevel=0.002)
    wx, _, _ = _extent(0.24, g)
    for sx in (-1, 1):
        x = CX + sx * 0.55 * wx
        yp = _front(0.24, g)
        S.box("jpocket", x - 0.04, yp - 0.005, 0.20, x + 0.04, yp + 0.01, 0.28, "cloth", bevel=0.006)
        S.box("jflap", x - 0.044, yp - 0.007, 0.27, x + 0.044, yp + 0.008, 0.30, "cloth", bevel=0.005)


def _tribalwear():
    """A one-piece tunic from shoulders to shins: a fringed hem, a cord belt
    with a bone toggle and a leather strap over one shoulder."""
    g = G + 0.004
    _shell("tunic", 0.0, TOP, g, "cloth", round_lo=True, round_hi=True)
    wx, yf, yb = _extent(0.06, g)
    for k in range(14):                                              # fringe tassels round the hem
        a = 2 * math.pi * (k + 0.5) / 14
        c, s = math.cos(a), math.sin(a)
        x = CX + c * wx * 0.94
        y = CY + s * (yb if s > 0 else yf) * 0.94
        S.ellipsoid("tassel", x, y, 0.07, 0.016, 0.016, 0.035, "cloth", segs=8, rings=5)
    _shell("cord", 0.24, 0.26, g + 0.004, "leather")
    y = _front(0.25, g)
    S.ellipsoid("toggle", CX + 0.04, y - 0.01, 0.25, 0.02, 0.012, 0.03, "horn", segs=10, rings=6)
    # a leather sash from the +X shoulder down across the chest to the -X hip:
    # a thin shell band cut by two parallel tilted planes
    sash = _shell("sash", 0.16, TOP - 0.005, g + 0.004, "leather")
    n = (0.62, 0.0, 1.0)
    bm = bmesh.new()
    bm.from_mesh(sash.data)
    for z, keep_above in ((0.36, True), (0.43, False)):
        no = n if keep_above else tuple(-c for c in n)
        bmesh.ops.bisect_plane(bm, geom=bm.verts[:] + bm.edges[:] + bm.faces[:], dist=1e-6,
                               plane_co=(CX, CY, z), plane_no=no, clear_inner=True)
    bm.to_mesh(sash.data)
    bm.free()
    sash.data.update()


def _cowboy_hat():
    """A wide-brimmed hat with upturned sides, a creased crown and a band."""
    H = HEAD_Z
    S.cyl("brim", CX, CY, 0.31, H + 0.06, H + 0.085, "cloth", segs=32, ry=0.29, bevel=0.008)
    for sx in (-1, 1):
        S.ellipsoid("curl", CX + sx * 0.285, CY, H + 0.095, 0.045, 0.22, 0.035, "cloth", segs=14, rings=8)
    S.cyl("crown", CX, CY, 0.195, H + 0.07, H + 0.20, "cloth", segs=28, ry=0.185, r_top=0.17, bevel=0.01)
    for sx in (-1, 1):                                               # two lobes leave a crease front to back
        S.ellipsoid("lobe", CX + sx * 0.07, CY, H + 0.20, 0.11, 0.17, 0.06, "cloth", segs=18, rings=10)
    S.cyl("band", CX, CY, 0.197, H + 0.085, H + 0.12, "leather", segs=28, ry=0.187, bevel=0.0)


def _bowler_hat():
    """A round-crowned hat with a narrow, slightly curled brim and a dark band."""
    H = HEAD_Z
    S.cyl("bbrim", CX, CY, 0.25, H + 0.06, H + 0.08, "cloth", segs=30, ry=0.235, bevel=0.008)
    S.cyl("rollw", CX, CY, 0.25, H + 0.075, H + 0.095, "cloth", segs=30, ry=0.235, r_top=0.245, bevel=0.006)
    S.cyl("bcrown", CX, CY, 0.195, H + 0.07, H + 0.16, "cloth", segs=28, ry=0.185, bevel=0.0)
    S.ellipsoid("dome", CX, CY, H + 0.16, 0.195, 0.185, 0.12, "cloth", segs=28, rings=14)
    S.cyl("bband", CX, CY, 0.197, H + 0.08, H + 0.115, "fur_black", segs=28, ry=0.187, bevel=0.0)


BODY = {"button_down": _button_down, "jacket": _jacket, "tribalwear": _tribalwear}
HATS = {"cowboy_hat": _cowboy_hat, "bowler_hat": _bowler_hat}


def _layer(name, facing, builder):
    """One worn layer over a body holdout, like pawn._layer; body garments
    are tagged per build (m_avg unsuffixed), hats are shared."""
    P._FACING[0] = facing
    S.holdout(P._body())
    builder()
    P._apply_modifiers()
    P._figure(facing)
    extra = {"layer": name, "facing": facing}
    tag = name
    if name in BODY and P._build_key != P.LEGACY_BUILD:
        tag = "%s_%s" % (name, P._build_key)
        extra["build"] = P._build_key
    return S.render_asset("pawn", "%s_%s" % (tag, facing), zmax=P._zmax_screen(), margins=P.MARGINS, extra=extra)


# ------------------------------------------------------------ ground items
def _folded_shirt(collar_wings):
    """A folded shirt: body, short folded sleeves at the shoulders, a collar."""
    S.box("body", 0.22, -0.80, 0.0, 0.78, -0.30, 0.08, "cloth", bevel=0.04, segments=2)
    S.box("fold", 0.26, -0.76, 0.08, 0.74, -0.46, 0.12, "cloth", bevel=0.03)
    for x0 in (0.14, 0.72):
        S.box("sleeve", x0, -0.44, 0.0, x0 + 0.14, -0.30, 0.07, "cloth", bevel=0.03)
    S.cyl("collar", 0.5, -0.33, 0.10, 0.06, 0.12, "cloth", segs=18, ry=0.06, bevel=0.01)
    if collar_wings:
        for sx in (-1, 1):
            S.prism("wing", [(0.5 + sx * 0.01, -0.38), (0.5 + sx * 0.10, -0.36), (0.5 + sx * 0.04, -0.44)],
                    0.10, 0.135, "cloth", bevel=0.006)


def _tshirt_item():
    _folded_shirt(False)
    return S.render_asset("item", "tshirt", zmax=0.18, extra={"item": "t-shirt"})


def _button_down_item():
    _folded_shirt(True)
    S.box("placket", 0.48, -0.76, 0.12, 0.52, -0.44, 0.13, "cloth", bevel=0.004)
    for y in (-0.70, -0.60, -0.50):
        S.cyl("button", 0.5, y, 0.014, 0.12, 0.14, "ceramic", segs=8, bevel=0.0)
    return S.render_asset("item", "button_down", zmax=0.18, extra={"item": "button-down shirt"})


def _pants_item():
    """Trousers folded over: two legs side by side under a belted waist."""
    for x0 in (0.22, 0.51):
        S.box("leg", x0, -0.84, 0.0, x0 + 0.27, -0.34, 0.08, "cloth", bevel=0.035, segments=2)
        S.box("cuff", x0 + 0.01, -0.86, 0.0, x0 + 0.26, -0.80, 0.09, "cloth", bevel=0.02)
    S.box("waist", 0.20, -0.38, 0.0, 0.80, -0.24, 0.10, "cloth", bevel=0.025)
    S.box("belt", 0.19, -0.33, 0.06, 0.81, -0.28, 0.115, "leather", bevel=0.01)
    S.box("buckle", 0.46, -0.345, 0.07, 0.54, -0.265, 0.125, "steel", bevel=0.006)
    return S.render_asset("item", "pants", zmax=0.16, extra={"item": "pants"})


def _jacket_item():
    S.box("body", 0.18, -0.80, 0.0, 0.82, -0.28, 0.10, "cloth", bevel=0.05, segments=2)
    S.box("hem", 0.18, -0.82, 0.0, 0.82, -0.74, 0.105, "cloth", bevel=0.02)
    S.box("zip", 0.49, -0.82, 0.08, 0.51, -0.30, 0.115, "metal_dark", bevel=0.003)
    for x in (0.32, 0.68):
        S.box("pocket", x - 0.07, -0.68, 0.08, x + 0.07, -0.56, 0.115, "cloth", bevel=0.01)
        S.box("flap", x - 0.075, -0.58, 0.09, x + 0.075, -0.54, 0.125, "cloth", bevel=0.008)
    S.cyl("collar", 0.5, -0.29, 0.12, 0.04, 0.14, "cloth", segs=18, ry=0.06, bevel=0.012)
    return S.render_asset("item", "jacket", zmax=0.18, extra={"item": "jacket"})


def _tribalwear_item():
    """A folded tunic with a fringed edge, a cord and a leather strap."""
    S.box("tunic", 0.20, -0.78, 0.0, 0.80, -0.26, 0.09, "cloth", bevel=0.05, segments=2)
    for k in range(9):
        x = 0.24 + k * 0.065
        S.ellipsoid("tassel", x, -0.82, 0.02, 0.018, 0.05, 0.02, "cloth", segs=8, rings=5)
    S.box("strap", 0.30, -0.70, 0.09, 0.38, -0.30, 0.11, "leather", bevel=0.008)
    S.box("cord", 0.20, -0.50, 0.08, 0.80, -0.47, 0.105, "leather", bevel=0.006)
    S.ellipsoid("toggle", 0.62, -0.485, 0.11, 0.03, 0.018, 0.02, "horn", segs=10, rings=6)
    return S.render_asset("item", "tribalwear", zmax=0.16, extra={"item": "tribalwear"})


def _cowboy_hat_item():
    S.cyl("brim", 0.5, -0.55, 0.34, 0.0, 0.025, "cloth", segs=32, ry=0.27, bevel=0.008)
    for sx in (-1, 1):
        S.ellipsoid("curl", 0.5 + sx * 0.31, -0.55, 0.04, 0.05, 0.2, 0.04, "cloth", segs=14, rings=8)
    S.cyl("crown", 0.5, -0.55, 0.19, 0.02, 0.15, "cloth", segs=28, ry=0.16, r_top=0.165, bevel=0.01)
    for sx in (-1, 1):
        S.ellipsoid("lobe", 0.5 + sx * 0.07, -0.55, 0.15, 0.11, 0.14, 0.05, "cloth", segs=18, rings=10)
    S.cyl("band", 0.5, -0.55, 0.192, 0.03, 0.065, "leather", segs=28, ry=0.162, bevel=0.0)
    return S.render_asset("item", "cowboy_hat", zmax=0.24, extra={"item": "cowboy hat"})


def _bowler_hat_item():
    S.cyl("brim", 0.5, -0.55, 0.26, 0.0, 0.025, "cloth", segs=30, ry=0.22, bevel=0.008)
    S.cyl("crown", 0.5, -0.55, 0.19, 0.02, 0.10, "cloth", segs=28, ry=0.16, bevel=0.0)
    S.ellipsoid("dome", 0.5, -0.55, 0.10, 0.19, 0.16, 0.12, "cloth", segs=28, rings=14)
    S.cyl("band", 0.5, -0.55, 0.192, 0.025, 0.06, "fur_black", segs=28, ry=0.162, bevel=0.0)
    return S.render_asset("item", "bowler_hat", zmax=0.26, extra={"item": "bowler hat"})


ITEMS = {"tshirt": _tshirt_item, "button_down": _button_down_item, "pants": _pants_item,
         "jacket": _jacket_item, "tribalwear": _tribalwear_item, "cowboy_hat": _cowboy_hat_item,
         "bowler_hat": _bowler_hat_item}


# The ui icon shows the garment in one cloth preset (base.yaml tints.cloth),
# so it is not a white shape in the HUD, which draws icons untinted.
ICON_TINT = {"tshirt": "#3f5170", "button_down": "#e3dccb", "pants": "#6b7480", "jacket": "#7a8150",
             "tribalwear": "#a2593a", "cowboy_hat": "#cbb48c", "bowler_hat": "#6b7480"}


def _tinted_icon(O, name):
    """ui_icon/<name>, cut from item/<name> with its mask multiplied by ICON_TINT."""
    import tempfile

    from ..palette import hex_to_srgb
    src = S.out_path("item", name)
    img, mask = S._read_png(src), S._read_png(S.out_path("item", name + "_mask"))
    k = mask[..., :1]
    rgb = np.array(hex_to_srgb(ICON_TINT[name]), np.float32)
    img[..., :3] = img[..., :3] * (1 - k) + img[..., :3] * rgb * k
    tmp = os.path.join(tempfile.gettempdir(), "rwg_icon_%s_%d.png" % (name, os.getpid()))
    S.write_png(img, tmp)
    return O._cut_icon(name, tmp, "%s:item/%s" % (C.NAMESPACE, name))


def _env(name):
    v = os.environ.get(name, "").strip()
    return [x for x in v.split(",") if x] or None


def build():
    only = _env("RWG_APPAREL")
    builds = _env("RWG_PAWN_BUILDS") or list(P.BUILD_KEYS)
    out = []
    try:
        for bk in builds:
            P.use_build(bk)
            for f in P.FACINGS:
                for name, fn in list(BODY.items()) + list(HATS.items()):
                    if only and name not in only:
                        continue
                    if name in HATS and bk != P.LEGACY_BUILD:
                        continue          # hats: one render, on m_avg
                    out.append(_layer(name, f, fn))
    finally:
        P.use_build(P.LEGACY_BUILD)
        P._FACING[0] = "south"
    from . import overlay as O
    for name, fn in ITEMS.items():
        if not only or name in only:
            out.append(fn())
            p = _tinted_icon(O, name)
            if p:
                out.append(p)
    return out
