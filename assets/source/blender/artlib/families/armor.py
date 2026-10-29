"""Vanilla-core armor (DEF-2, ART §10, §15): worn layers + ground sprites.

Worn layers use the pawn rig (`pawn.py`): the same limbless capsule, six
builds, S/E/N facings (west is east mirrored), rendered over a body
holdout so any layer subset composites. Keys follow the pawn family:
    pawn/<layer>_<build>_<facing>   body armor, every build (m_avg unsuffixed)
    pawn/<helmet>_<facing>          helmets, shared by every build, like tuque_knit
Draw order on the pawn: ... shirt -> flak_pants -> flak_vest / flak_jacket
-> plate_armor / marine_armor -> beard -> hair -> helmet.
Ground sprites are item/<kind with _> (ItemSprite's fallback finds them).
Stuff-made pieces (plate armor, simple and advanced helmet) put their metal
in the neutral `cloth` group so the renderer can multiply the stuff colour
over the mask; untinted they read as plain steel. Flak and marine pieces
keep fixed colours, like the parka. All shapes are original.
    make art FAMILIES=armor        (RWG_ARMOR=flak_vest,simple_helmet and
                                    RWG_PAWN_BUILDS=m_avg narrow a review build)
"""
import os

from .. import stage as S
from . import pawn as P
from .pawn import CX, CY, G, HEAD_R, HEAD_Z, TOP, _extent, _hair_shell, _plane, _shell, _vplane

# ------------------------------------------------------------ worn layers


def _front(z, g):
    """Front surface y of the shell grown by g at height z."""
    return CY - _extent(z, g)[1]


def _flak_pants():
    """Olive padded trousers over the hips with knee pads and thigh pouches."""
    g = G + 0.014
    _shell("fpants", 0.0, 0.29, g, "fur_moss", round_lo=True)
    _shell("fbelt", 0.22, 0.27, g + 0.01, "fur_black")
    wx, _, _ = _extent(0.12, g)
    for sx in (-1, 1):
        x = CX + sx * 0.45 * wx
        y = _front(0.07, g)
        S.box("kneepad", x - 0.045, y - 0.02, 0.03, x + 0.045, y + 0.03, 0.12, "fur_black", bevel=0.014)
        xs = CX + sx * (wx + 0.004)
        S.box("pouch", xs - 0.022, CY - 0.05, 0.12, xs + 0.022, CY + 0.05, 0.2, "fur_moss", bevel=0.01)


def _flak_vest():
    """Bulky olive vest from the waist to the shoulders: high collar, a
    raised chest plate and three magazine pouches."""
    g = G + 0.032
    _shell("fvest", 0.20, TOP - 0.02, g, "fur_moss")
    S.cyl("fcollar", CX, CY, 0.125, TOP - 0.05, TOP + 0.05, "fur_moss", segs=20, ry=0.11, bevel=0.012)
    wx, yf, yb = _extent(0.40, g)
    S.box("plate", CX - 0.62 * wx, CY - yf - 0.016, 0.33, CX + 0.62 * wx, CY - yf + 0.02, TOP - 0.04,
          "fur_moss", bevel=0.018)
    for k in (-1, 0, 1):
        x = CX + k * 0.075
        y = _front(0.28, g)
        S.box("pouch", x - 0.032, y - 0.03, 0.22, x + 0.032, y + 0.01, 0.32, "fur_black", bevel=0.01)
    for sx in (-1, 1):
        x = CX + sx * 0.55 * wx
        S.box("strap", x - 0.03, CY - yf - 0.01, TOP - 0.08, x + 0.03, CY + yb + 0.01, TOP + 0.03, "fur_moss",
              bevel=0.01)
    _shell("vband", 0.20, 0.24, g + 0.008, "fur_black")


def _flak_jacket():
    """Quilted grey jacket over the torso and shoulders: padded bands, a
    front zip, olive shoulder pads and a stand collar."""
    g = G + 0.022
    _shell("fjacket", 0.12, TOP, g, "fur_wolf", round_hi=True)
    for z in (0.20, 0.30, 0.40):
        _shell("quilt", z, z + 0.022, g + 0.007, "fur_taupe")
    _shell("jhem", 0.12, 0.16, g + 0.01, "fur_taupe")
    y = _front(0.3, g)
    S.box("zip", CX - 0.01, y - 0.016, 0.13, CX + 0.01, y + 0.01, TOP - 0.02, "metal_dark", bevel=0.003)
    S.cyl("jcollar", CX, CY, 0.11, TOP - 0.03, TOP + 0.04, "fur_wolf", segs=18, ry=0.095, bevel=0.01)
    wx, _, _ = _extent(TOP - 0.06, g)
    for sx in (-1, 1):
        S.ellipsoid("pad", CX + sx * (wx - 0.03), CY, TOP - 0.07, 0.07, 0.095, 0.06, "fur_moss", segs=16, rings=8)


def _plate_armor():
    """Full plate over the body (the head stays bare): breastplate, banded
    skirt, round pauldrons and a gorget. Metal is the tintable stuff."""
    g = G + 0.03
    _shell("plate", 0.0, TOP, g, "cloth", round_lo=True, round_hi=True)
    _shell("breast", 0.30, TOP - 0.03, g + 0.014, "cloth")
    for z in (0.06, 0.12, 0.18):
        _shell("lame", z, z + 0.03, g + 0.012, "cloth")
    _shell("pbelt", 0.235, 0.27, g + 0.02, "leather")
    y = _front(0.25, g + 0.02)
    S.box("buckle", CX - 0.03, y - 0.014, 0.23, CX + 0.03, y + 0.01, 0.275, "gold", bevel=0.006)
    y = _front(0.42, g + 0.014)
    S.box("ridge", CX - 0.012, y - 0.014, 0.31, CX + 0.012, y + 0.012, TOP - 0.05, "cloth", bevel=0.006)
    S.cyl("gorget", CX, CY, 0.12, TOP - 0.04, TOP + 0.045, "cloth", segs=20, ry=0.105, bevel=0.012)
    wx, _, _ = _extent(TOP - 0.05, g)
    for sx in (-1, 1):
        S.ellipsoid("pauldron", CX + sx * (wx - 0.02), CY, TOP - 0.05, 0.085, 0.11, 0.07, "cloth", segs=18,
                    rings=10)
        S.ellipsoid("pauldron2", CX + sx * (wx + 0.005), CY, TOP - 0.11, 0.07, 0.1, 0.05, "cloth", segs=16,
                    rings=8)


def _marine_armor():
    """Bulky powered armor head to toe: blue-grey shell, dark joints, a
    chest plate, heavy pauldrons and a power pack on the back."""
    g = G + 0.05
    _shell("marine", 0.0, TOP, g, "fur_steel", round_lo=True, round_hi=True)
    for z in (0.10, 0.25):
        _shell("joint", z, z + 0.035, g + 0.008, "metal_dark")
    _shell("chest", 0.31, TOP - 0.04, g + 0.018, "steel")
    y = _front(0.40, g + 0.018)
    S.box("vent", CX - 0.05, y - 0.012, 0.36, CX + 0.05, y + 0.01, 0.44, "metal_dark", bevel=0.008)
    for k in (-1, 1):
        S.box("light", CX + k * 0.075 - 0.012, y - 0.014, 0.47, CX + k * 0.075 + 0.012, y + 0.008, 0.5,
              "crystal_blue", bevel=0.004)
    S.cyl("mcollar", CX, CY, 0.13, TOP - 0.03, TOP + 0.05, "metal_dark", segs=20, ry=0.115, bevel=0.012)
    wx, _, yb = _extent(TOP - 0.06, g)
    for sx in (-1, 1):
        S.ellipsoid("mpauldron", CX + sx * (wx - 0.01), CY, TOP - 0.06, 0.1, 0.13, 0.085, "fur_steel", segs=18,
                    rings=10)
        S.ellipsoid("mrim", CX + sx * (wx + 0.02), CY, TOP - 0.11, 0.07, 0.12, 0.04, "metal_dark", segs=16,
                    rings=8)
    _, _, yb = _extent(0.38, g)
    S.box("pack", CX - 0.12, CY + yb - 0.02, 0.26, CX + 0.12, CY + yb + 0.08, TOP - 0.01, "metal_dark",
          bevel=0.02)
    for k in (-1, 1):
        S.cyl("exhaust", CX + k * 0.07, CY + yb + 0.04, 0.026, TOP - 0.02, TOP + 0.04, "steel", segs=10,
              bevel=0.004)


def _simple_helmet():
    """A round steel cap with a narrow brim and a chin strap (stuff metal)."""
    H = HEAD_Z
    _hair_shell("shelm", 0.045, [_plane(H + 0.03, H - 0.02)], mat="cloth")
    S.cyl("brim", CX, CY, 0.255, H + 0.015, H + 0.04, "cloth", segs=28, ry=0.24, bevel=0.008)
    S.ellipsoid("rivet", CX, CY - 0.2, H + 0.1, 0.018, 0.012, 0.018, "leather", segs=8, rings=5)
    for sx in (-1, 1):
        S.box("strap", CX + sx * 0.19 - 0.012, CY - 0.06, H - 0.16, CX + sx * 0.19 + 0.012, CY + 0.0, H + 0.02,
              "leather", bevel=0.004)


def _advanced_helmet():
    """A deep composite helmet with flared neck guard, ear covers and a
    smoked visor across the eyes (stuff shell)."""
    H = HEAD_Z
    _hair_shell("ahelm", 0.05, [_plane(H + 0.02, H - 0.13)], mat="cloth")
    _hair_shell("visor", 0.058, [_plane(H - 0.07, H - 0.07), _plane(H + 0.03, H + 0.03, above=False),
                                 _vplane(CY - 0.06)], mat="glass")
    S.box("vband", CX - 0.2, CY - 0.06, H + 0.03, CX + 0.2, CY + 0.06, H + 0.06, "cloth", bevel=0.01)
    for sx in (-1, 1):
        S.ellipsoid("ear", CX + sx * 0.215, CY + 0.01, H - 0.04, 0.035, 0.07, 0.07, "cloth", segs=14, rings=8)
    S.box("mount", CX - 0.03, CY - 0.235, H + 0.08, CX + 0.03, CY - 0.19, H + 0.13, "metal_dark", bevel=0.008)


def _marine_helmet():
    """A sealed helmet: blue-grey dome over the whole head, a glowing visor
    slit, a breathing grille and a low crest."""
    H = HEAD_Z
    h = 1.0
    S.ellipsoid("mhelm", CX, CY, H, HEAD_R[0] * h + 0.055, HEAD_R[1] * h + 0.055, HEAD_R[2] * h + 0.05,
                "fur_steel", segs=36, rings=20)
    _hair_shell("mvisor", 0.062, [_plane(H - 0.02, H - 0.02), _plane(H + 0.05, H + 0.05, above=False),
                                  _vplane(CY - 0.09)], mat="crystal_blue")
    _hair_shell("grille", 0.062, [_plane(H - 0.17, H - 0.17), _plane(H - 0.07, H - 0.07, above=False),
                                  _vplane(CY - 0.14)], mat="metal_dark")
    S.box("crest", CX - 0.022, CY - 0.18, H + 0.16, CX + 0.022, CY + 0.16, H + 0.26, "metal_dark", bevel=0.012)
    S.cyl("mneck", CX, CY, 0.17, H - 0.26, H - 0.18, "metal_dark", segs=20, ry=0.15, bevel=0.01)


BODY = {"flak_pants": _flak_pants, "flak_vest": _flak_vest, "flak_jacket": _flak_jacket,
        "plate_armor": _plate_armor, "marine_armor": _marine_armor}
HELMETS = {"simple_helmet": _simple_helmet, "advanced_helmet": _advanced_helmet, "marine_helmet": _marine_helmet}


def _layer(name, facing, builder):
    """One worn layer over a body holdout, like pawn._layer; body armor is
    tagged per build (m_avg unsuffixed), helmets are shared."""
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
def _flak_vest_item():
    S.box("body", 0.20, -0.80, 0.0, 0.80, -0.30, 0.12, "fur_moss", bevel=0.05, segments=2)
    S.box("plate", 0.30, -0.70, 0.12, 0.70, -0.40, 0.16, "fur_moss", bevel=0.03)
    S.cyl("collar", 0.5, -0.30, 0.13, 0.04, 0.16, "fur_moss", segs=18, ry=0.07, bevel=0.012)
    for k in (-1, 0, 1):
        x = 0.5 + k * 0.11
        S.box("pouch", x - 0.045, -0.80, 0.02, x + 0.045, -0.74, 0.15, "fur_black", bevel=0.012)
    S.box("band", 0.20, -0.66, 0.0, 0.80, -0.62, 0.125, "fur_black", bevel=0.008)
    return S.render_asset("item", "flak_vest", zmax=0.2, extra={"item": "flak vest"})


def _flak_pants_item():
    for x0 in (0.20, 0.52):
        S.box("leg", x0, -0.84, 0.0, x0 + 0.28, -0.32, 0.09, "fur_moss", bevel=0.04, segments=2)
        S.box("kneepad", x0 + 0.05, -0.66, 0.09, x0 + 0.23, -0.52, 0.13, "fur_black", bevel=0.02)
    S.box("waist", 0.20, -0.36, 0.0, 0.80, -0.24, 0.11, "fur_black", bevel=0.02)
    return S.render_asset("item", "flak_pants", zmax=0.16, extra={"item": "flak pants"})


def _flak_jacket_item():
    S.box("body", 0.18, -0.80, 0.0, 0.82, -0.28, 0.12, "fur_wolf", bevel=0.05, segments=2)
    for y in (-0.70, -0.58, -0.46):
        S.box("quilt", 0.18, y, 0.0, 0.82, y + 0.03, 0.128, "fur_taupe", bevel=0.01)
    S.box("zip", 0.49, -0.80, 0.1, 0.51, -0.30, 0.135, "metal_dark", bevel=0.003)
    for x in (0.20, 0.80):
        S.ellipsoid("pad", x, -0.36, 0.1, 0.09, 0.08, 0.06, "fur_moss", segs=14, rings=8)
    S.cyl("collar", 0.5, -0.28, 0.12, 0.05, 0.15, "fur_wolf", segs=18, ry=0.06, bevel=0.012)
    return S.render_asset("item", "flak_jacket", zmax=0.2, extra={"item": "flak jacket"})


def _plate_armor_item():
    """A cuirass lying face up: breastplate, skirt lames, a pauldron beside it."""
    S.box("cuirass", 0.24, -0.78, 0.0, 0.68, -0.30, 0.12, "cloth", bevel=0.06, segments=2)
    S.ellipsoid("breast", 0.46, -0.50, 0.08, 0.19, 0.17, 0.08, "cloth", segs=20, rings=10)
    S.box("ridge", 0.45, -0.70, 0.1, 0.47, -0.34, 0.165, "cloth", bevel=0.008)
    S.box("gorget", 0.34, -0.33, 0.06, 0.58, -0.26, 0.15, "cloth", bevel=0.03)
    for k, y in enumerate((-0.84, -0.90)):
        S.box("lame%d" % k, 0.26 + 0.03 * k, y, 0.0, 0.66 - 0.03 * k, y + 0.07, 0.06, "cloth", bevel=0.015)
    S.box("belt", 0.24, -0.78, 0.0, 0.68, -0.72, 0.13, "leather", bevel=0.01)
    S.ellipsoid("pauldron", 0.80, -0.44, 0.04, 0.10, 0.13, 0.08, "cloth", segs=16, rings=8)
    return S.render_asset("item", "plate_armor", zmax=0.2, extra={"item": "plate armor"})


def _marine_armor_item():
    S.box("torso", 0.24, -0.80, 0.0, 0.76, -0.30, 0.16, "fur_steel", bevel=0.06, segments=2)
    S.box("chest", 0.32, -0.66, 0.16, 0.68, -0.36, 0.2, "steel", bevel=0.03)
    S.box("vent", 0.44, -0.62, 0.2, 0.56, -0.52, 0.215, "metal_dark", bevel=0.008)
    for x in (0.24, 0.76):
        S.ellipsoid("pauldron", x, -0.38, 0.1, 0.11, 0.12, 0.09, "fur_steel", segs=16, rings=8)
    S.box("joint", 0.24, -0.82, 0.0, 0.76, -0.76, 0.12, "metal_dark", bevel=0.02)
    for k in (-1, 1):
        S.box("light", 0.5 + k * 0.1 - 0.015, -0.42, 0.2, 0.5 + k * 0.1 + 0.015, -0.38, 0.215, "crystal_blue",
              bevel=0.004)
    return S.render_asset("item", "marine_armor", zmax=0.26, extra={"item": "marine armor"})


def _simple_helmet_item():
    S.ellipsoid("dome", 0.5, -0.55, 0.04, 0.2, 0.19, 0.17, "cloth", segs=22, rings=12)
    S.cyl("brim", 0.5, -0.55, 0.26, 0.02, 0.06, "cloth", segs=28, ry=0.245, bevel=0.01)
    S.box("strap", 0.30, -0.80, 0.0, 0.70, -0.76, 0.03, "leather", bevel=0.008)
    return S.render_asset("item", "simple_helmet", zmax=0.24, extra={"item": "simple helmet"})


def _advanced_helmet_item():
    S.ellipsoid("dome", 0.5, -0.52, 0.02, 0.21, 0.22, 0.19, "cloth", segs=22, rings=12)
    S.box("visor", 0.32, -0.76, 0.06, 0.68, -0.62, 0.13, "glass", bevel=0.03)
    for x in (0.29, 0.71):
        S.ellipsoid("ear", x, -0.52, 0.08, 0.04, 0.08, 0.07, "cloth", segs=12, rings=6)
    S.box("mount", 0.47, -0.72, 0.15, 0.53, -0.66, 0.2, "metal_dark", bevel=0.008)
    return S.render_asset("item", "advanced_helmet", zmax=0.26, extra={"item": "advanced helmet"})


def _marine_helmet_item():
    S.ellipsoid("dome", 0.5, -0.52, 0.0, 0.23, 0.23, 0.22, "fur_steel", segs=24, rings=12)
    S.box("visor", 0.34, -0.76, 0.08, 0.66, -0.68, 0.14, "crystal_blue", bevel=0.02)
    S.box("grille", 0.40, -0.76, 0.0, 0.60, -0.68, 0.06, "metal_dark", bevel=0.015)
    S.box("crest", 0.48, -0.70, 0.18, 0.52, -0.32, 0.25, "metal_dark", bevel=0.012)
    return S.render_asset("item", "marine_helmet", zmax=0.28, extra={"item": "marine helmet"})


ITEMS = {"flak_vest": _flak_vest_item, "flak_pants": _flak_pants_item, "flak_jacket": _flak_jacket_item,
         "plate_armor": _plate_armor_item, "marine_armor": _marine_armor_item,
         "simple_helmet": _simple_helmet_item, "advanced_helmet": _advanced_helmet_item,
         "marine_helmet": _marine_helmet_item}


def _env(name):
    v = os.environ.get(name, "").strip()
    return [x for x in v.split(",") if x] or None


def build():
    only = _env("RWG_ARMOR")
    builds = _env("RWG_PAWN_BUILDS") or list(P.BUILD_KEYS)
    out = []
    try:
        for bk in builds:
            P.use_build(bk)
            for f in P.FACINGS:
                for name, fn in list(BODY.items()) + list(HELMETS.items()):
                    if only and name not in only:
                        continue
                    if name in HELMETS and bk != P.LEGACY_BUILD:
                        continue          # helmets: one render, on m_avg
                    out.append(_layer(name, f, fn))
    finally:
        P.use_build(P.LEGACY_BUILD)
        P._FACING[0] = "south"
    for name, fn in ITEMS.items():
        if not only or name in only:
            out.append(fn())
    return out
