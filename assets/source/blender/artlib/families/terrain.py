"""Terrain (ART spec §8-§9).

Ground tiles are 2D, generated with numpy so they are exactly tileable:
lattice values on tile borders are shared by every variant of a terrain
type, so any variant can sit next to any other without a seam. Colours
are quantised to the material's three palette bands, low frequency, so
terrain never competes with pawns. Rock chunks are 3D (stage render).
"""
import numpy as np

from .. import contracts as C
from .. import meta, palette
from .. import stage as S

N = C.PPT
VARIANTS = 4


def _rgb(h):
    return np.array(palette.hex_to_srgb(h), np.float32)


def _lattice(cells, border_seed, seed):
    rb = np.random.RandomState(border_seed)
    ri = np.random.RandomState(seed)
    g = ri.uniform(-1, 1, (cells + 1, cells + 1))
    edge = rb.uniform(-1, 1, (4, cells + 1))
    corner = rb.uniform(-1, 1)
    g[0, :], g[cells, :] = edge[0], edge[0]          # north == south (period 1 tile)
    g[:, 0], g[:, cells] = edge[1], edge[1]          # west == east
    g[0, 0] = g[0, cells] = g[cells, 0] = g[cells, cells] = corner
    return g


def _noise(cells, border_seed, seed):
    g = _lattice(cells, border_seed, seed)
    t = (np.arange(N) + 0.5) / N * cells
    i = np.floor(t).astype(int)
    f = t - i
    f = f * f * (3 - 2 * f)
    a = g[i][:, i]
    b = g[i][:, i + 1]
    c = g[i + 1][:, i]
    d = g[i + 1][:, i + 1]
    fx, fy = f[None, :], f[:, None]
    return (a * (1 - fx) + b * fx) * (1 - fy) + (c * (1 - fx) + d * fx) * fy


SOFT = 0.45  # terrain bands sit closer to base than object bands (ART §8)


def _soft(p, band):
    return _rgb(p["base"]) * (1 - SOFT) + _rgb(p[band]) * SOFT


def _bands(field, p, lo, hi):
    img = np.empty((N, N, 4), np.float32)
    img[..., 3] = 1
    img[..., :3] = _rgb(p["base"])
    img[field < lo, :3] = _soft(p, "shadow")
    img[field > hi, :3] = _soft(p, "light")
    return img


def _stamp(img, rng, count, rmin, rmax, p, squash=0.8):
    """Pebbles/clods: lit upper-left rim, shadow lower-right. Wrap-around."""
    yy, xx = np.mgrid[0:N, 0:N]
    for _ in range(count):
        cx, cy = rng.uniform(0, N, 2)
        r = rng.uniform(rmin, rmax)
        for ox in (-N, 0, N):
            for oy in (-N, 0, N):
                dx = (xx - cx - ox) / r
                dy = (yy - cy - oy) / (r * squash)
                d = dx * dx + dy * dy
                sh = ((xx - cx - ox - r * 0.25) / r) ** 2 + ((yy - cy - oy - r * 0.3) / (r * squash)) ** 2
                img[(sh < 1.0) & (d >= 1.0), :3] = _soft(p, "shadow")
                img[d < 1.0, :3] = _rgb(p["base"])
                img[(d < 1.0) & (((xx - cx - ox + r * 0.3) / r) ** 2 + ((yy - cy - oy + r * 0.3) / r) ** 2 < 0.35), :3] = _soft(p, "light")


def _cracks(img, rng, count, p):
    for _ in range(count):
        x, y = rng.uniform(0, N, 2)
        ang = rng.uniform(0, np.pi)
        for _ in range(int(rng.uniform(18, 40))):
            ang += rng.uniform(-0.5, 0.5)
            x, y = (x + np.cos(ang) * 1.5) % N, (y + np.sin(ang) * 1.5) % N
            img[int(y) % N, int(x) % N, :3] = _soft(p, "shadow")


SPECS = {
    # name: (material, lattice cells, (lo, hi) band thresholds, decorations)
    "soil":      ("soil", 4, (-0.62, 0.68), {"clods": (3, 2.0, 3.0)}),
    "soil_rich": ("soil_rich", 4, (-0.58, 0.66), {"clods": (6, 1.6, 2.6)}),
    "gravel":    ("gravel", 4, (-0.58, 0.60), {"pebbles": (18, 2.5, 4.5)}),
    "stone":     ("stone", 4, (-0.55, 0.60), {"cracks": 2, "pebbles": (3, 3.0, 5.0)}),
}


def _tile_img(name, v):
    mats, _ = S.pal()
    mat, cells, (lo, hi), deco = SPECS[name]
    p = mats[mat]
    border = 1000 + sum(map(ord, name))
    seed = border * 31 + v
    field = 0.70 * _noise(cells, border, seed) + 0.30 * _noise(cells * 2, border + 7, seed + 7)
    img = _bands(field, p, lo, hi)
    rng = np.random.RandomState(seed + 99)
    if "clods" in deco:
        _stamp(img, rng, *deco["clods"], p)
    if "pebbles" in deco:
        _stamp(img, rng, *deco["pebbles"], mats["stone"] if name == "gravel" else p)
    if "cracks" in deco:
        _cracks(img, rng, deco["cracks"], p)
    return img


def _tile(name, v):
    img = _tile_img(name, v)
    out = "%s_%02d" % (name, v)
    path = S.out_path("terrain", out)
    S.write_png(img, path)
    meta.write("terrain", out, {
        "image": "generated/world/terrain/%s.png" % out, "scale": C.SCALE,
        "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0],
        "tileable": True, "terrain": name, "variant": v, "variants": VARIANTS,
    })
    return path


# Ground transitions: a higher-priority terrain draws a soft, noisy edge
# over its lower-priority neighbours. Mask bits N=1, E=2, S=4, W=8 say
# which sides of the lower tile touch the higher terrain.
PRIORITY = ("soil", "soil_rich", "gravel", "stone")
EDGE_DEPTH = 30.0   # px @2x
EDGE_WOBBLE = 9.0


def _edge_img(name, mask):
    """Variant 0 of name with a soft, noisy alpha falling off away from
    the sides in mask."""
    border = 1000 + sum(map(ord, name))
    base = _tile_img(name, 0)
    wob = _noise(6, border + 13, border * 7 + mask) * EDGE_WOBBLE
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32) + 0.5
    dist = np.full((N, N), 1e9, np.float32)
    if mask & 1:
        dist = np.minimum(dist, yy)
    if mask & 2:
        dist = np.minimum(dist, N - xx)
    if mask & 4:
        dist = np.minimum(dist, N - yy)
    if mask & 8:
        dist = np.minimum(dist, xx)
    a = np.clip((EDGE_DEPTH + wob - dist) / 6.0, 0.0, 1.0)
    img = base.copy()
    img[..., 3] = a
    return img


def _edge(name, mask):
    img = _edge_img(name, mask)
    out = "%s_edge_%02d" % (name, mask)
    path = S.out_path("terrain", out)
    S.write_png(img, path)
    meta.write("terrain", out, {
        "image": "generated/world/terrain/%s.png" % out, "scale": C.SCALE,
        "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0],
        "transition": name, "mask": mask,
    })
    return path


def _rock(i):
    seeds = (11, 23, 37)
    sizes = (0.30, 0.22, 0.26)
    r = sizes[i]
    S.blob("rock", 0.5, -0.52, 0.0, r, "stone", seed=seeds[i], lumps=0.25, squash=0.8)
    if i != 1:
        S.blob("rock2", 0.5 + r * 0.9, -0.62, 0.0, r * 0.45, "stone_dark", seed=seeds[i] + 1, lumps=0.3)
    return S.render_asset("terrain", "rock_chunk_%02d" % i, zmax=r * 0.9,
                          extra={"decoration": True, "material": "stone"})


def build():
    out = []
    for name in SPECS:
        for v in range(VARIANTS):
            out.append(_tile(name, v))
    for i in range(3):
        out.append(_rock(i))
    for name in PRIORITY[1:]:
        for mask in range(1, 16):
            out.append(_edge(name, mask))
    return out
