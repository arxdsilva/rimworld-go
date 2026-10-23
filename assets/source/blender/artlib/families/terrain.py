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


def _joints(img, p):
    """Smoothed floor: faint flagstone joints, periodic so the tile stays seamless."""
    col = _soft(p, "shadow")
    hi = _soft(p, "light")
    for y in (0, N // 2):
        img[y, :, :3] = col
        img[(y + 1) % N, :, :3] = hi
    for x, ys in ((N // 4, (0, N // 2)), (3 * N // 4, (N // 2, N))):
        img[ys[0]:ys[1], x, :3] = col
        img[ys[0]:ys[1], (x + 1) % N, :3] = hi


def _cracks(img, rng, count, p, strong=False):
    for _ in range(count):
        x, y = rng.uniform(0, N, 2)
        ang = rng.uniform(0, np.pi)
        step = 0.7 if strong else 1.5
        for _ in range(int(rng.uniform(18, 40) * (2.2 if strong else 1))):
            ang += rng.uniform(-0.5, 0.5) * (0.5 if strong else 1)
            x, y = (x + np.cos(ang) * step) % N, (y + np.sin(ang) * step) % N
            col = _rgb(p["shadow"]) if strong else _soft(p, "shadow")
            img[int(y) % N, int(x) % N, :3] = col
            if strong:
                img[(int(y) + 1) % N, int(x) % N, :3] = col


def _glints(img, rng, count, p, water):
    """Marsh: small puddles with a pale glint on their upper-left rim."""
    yy, xx = np.mgrid[0:N, 0:N]
    for _ in range(count):
        cx, cy = rng.uniform(0, N, 2)
        rx, ry = rng.uniform(5, 11), rng.uniform(3, 6)
        for ox in (-N, 0, N):
            for oy in (-N, 0, N):
                d = ((xx - cx - ox) / rx) ** 2 + ((yy - cy - oy) / ry) ** 2
                img[d < 1.0, :3] = _rgb(water["shadow"]) * 0.6 + _rgb(water["base"]) * 0.4
                inner = ((xx - cx - ox) / (rx * 0.75)) ** 2 + ((yy - cy - oy) / (ry * 0.7)) ** 2
                img[(d < 1.0) & (inner < 1.0), :3] = _rgb(water["base"])
                gl = ((xx - cx - ox + rx * 0.3) / (rx * 0.35)) ** 2 + ((yy - cy - oy + ry * 0.3) / (ry * 0.28)) ** 2
                img[gl < 1.0, :3] = _rgb(water["light"])


def _ripples(img, rng, count, p):
    """Sand: short wind-ripple dashes (shadow line under a light line)."""
    for _ in range(count):
        x, y = rng.uniform(0, N, 2)
        ln = int(rng.uniform(12, 26))
        for i in range(ln):
            yy = (int(y + np.sin(i * 0.35) * 1.5)) % N
            xx = (int(x) + i) % N
            img[yy, xx, :3] = _soft(p, "light")
            img[(yy + 2) % N, xx, :3] = _soft(p, "shadow")


def _frost(img, rng, count, p):
    """Ice: pale frosty patches."""
    yy, xx = np.mgrid[0:N, 0:N]
    for _ in range(count):
        cx, cy = rng.uniform(0, N, 2)
        r = rng.uniform(4, 9)
        for ox in (-N, 0, N):
            for oy in (-N, 0, N):
                d = ((xx - cx - ox) / r) ** 2 + ((yy - cy - oy) / (r * 0.7)) ** 2
                img[d < 1.0, :3] = _rgb(p["light"]) * 0.7 + _rgb(p["base"]) * 0.3


def _waves(img, rng, count, p, length=(14, 32), amp=2.2, thick=2):
    """Water: short wavy crest lines (pale line over a shadow line).
    Wrap-around, so every variant stays seamless."""
    hi = _rgb(p["light"]) * 0.75 + _rgb(p["base"]) * 0.25
    lo = _soft(p, "shadow")
    for _ in range(count):
        x, y = rng.uniform(0, N, 2)
        ln = int(rng.uniform(*length))
        ph = rng.uniform(0, 6.28)
        for i in range(ln):
            fade = min(1.0, i / 4.0, (ln - i) / 4.0)
            if fade < 0.5:
                continue
            xx = int(x + i) % N
            yy = int(y + np.sin(i * 0.32 + ph) * amp) % N
            for t in range(thick):
                img[(yy + t) % N, xx, :3] = hi
            img[(yy + thick + 1) % N, xx, :3] = lo


def _sparkle(img, rng, count, p):
    """Deep water: a few tiny pale cross glints."""
    hi = _rgb(p["light"])
    for _ in range(count):
        x, y = (int(v) for v in rng.uniform(0, N, 2))
        for dx, dy in ((0, 0), (1, 0), (-1, 0), (0, 1), (0, -1)):
            img[(y + dy) % N, (x + dx) % N, :3] = hi


def _shoals(img, rng, count, p, bottom):
    """Shallow water: sandy bottom patches showing through (lighter, warmer)."""
    yy, xx = np.mgrid[0:N, 0:N]
    tone = _rgb(p["base"]) * 0.55 + _rgb(p["light"]) * 0.30 + _rgb(bottom["light"]) * 0.15
    for _ in range(count):
        cx, cy = rng.uniform(0, N, 2)
        rx, ry = rng.uniform(16, 30), rng.uniform(9, 16)
        for ox in (-N, 0, N):
            for oy in (-N, 0, N):
                d = ((xx - cx - ox) / rx) ** 2 + ((yy - cy - oy) / ry) ** 2
                img[d < 1.0, :3] = tone


def _puddles(img, rng, count, p, water):
    """Mud: wet sheens with a pale glint, wider and flatter than marsh pools."""
    yy, xx = np.mgrid[0:N, 0:N]
    for _ in range(count):
        cx, cy = rng.uniform(0, N, 2)
        rx, ry = rng.uniform(13, 24), rng.uniform(6, 11)
        for ox in (-N, 0, N):
            for oy in (-N, 0, N):
                d = ((xx - cx - ox) / rx) ** 2 + ((yy - cy - oy) / ry) ** 2
                img[d < 1.0, :3] = _rgb(p["shadow"]) * 0.7 + _rgb(water["base"]) * 0.3
                inner = ((xx - cx - ox) / (rx * 0.72)) ** 2 + ((yy - cy - oy) / (ry * 0.62)) ** 2
                img[(d < 1.0) & (inner < 1.0), :3] = _rgb(p["shadow"]) * 0.45 + _rgb(water["base"]) * 0.55
                gl = ((xx - cx - ox + rx * 0.28) / (rx * 0.3)) ** 2 + ((yy - cy - oy + ry * 0.3) / (ry * 0.22)) ** 2
                img[gl < 1.0, :3] = _rgb(water["light"])


def _scuffs(img, rng, count, p):
    """Track: short dark drag marks and pale chips where feet and carts wore the dirt."""
    for _ in range(count):
        x, y = rng.uniform(0, N, 2)
        ang = rng.uniform(0, np.pi)
        for i in range(int(rng.uniform(8, 18))):
            xx = int(x + np.cos(ang) * i) % N
            yy = int(y + np.sin(ang) * i) % N
            img[yy, xx, :3] = _soft(p, "shadow")
            img[(yy + 1) % N, xx, :3] = _soft(p, "light")


SPECS = {
    # name: (material, lattice cells, (lo, hi) band thresholds, decorations)
    "soil":      ("soil", 4, (-0.62, 0.68), {"clods": (3, 2.0, 3.0)}),
    "soil_rich": ("soil_rich", 4, (-0.58, 0.66), {"clods": (6, 1.6, 2.6)}),
    "gravel":    ("gravel", 4, (-0.58, 0.60), {"pebbles": (18, 2.5, 4.5)}),
    # stone and its smoothed variant are tint-neutral: the renderer multiplies the cell's stone colour
    "stone":     ("stone_neutral", 4, (-0.55, 0.60), {"cracks": 2, "pebbles": (3, 3.0, 5.0)}),
    "stone_smooth": ("stone_neutral", 3, (-0.70, 0.74), {"joints": True}),
    # biome grounds
    "sand":      ("sand", 4, (-0.50, 0.55), {"ripples": 5}),
    "marsh":     ("marsh", 4, (-0.45, 0.55), {"glints": 9}),
    "ice":       ("ice", 3, (-0.50, 0.55), {"cracks": 6, "frost": 5}),
    # NG-1 water and ground (spec §8): walk 300 / impassable / walk 140 / walk 90
    "shallow_water": ("water_shallow", 3, (-0.55, 0.62), {"shoals": 3, "waves": (14, 2.0)}),
    "deep_water":    ("water_deep", 3, (-0.50, 0.58), {"waves": (7, 1.6), "sparkle": 6}),
    "mud":           ("mud", 4, (-0.50, 0.58), {"puddles": 3, "clods": (5, 1.4, 2.4)}),
    "track":         ("track_dirt", 4, (-0.55, 0.62), {"pebbles": (12, 1.8, 3.2), "scuffs": 9}),
}
LIQUIDS = ("shallow_water", "deep_water")


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
        _stamp(img, rng, *deco["pebbles"], mats["gravel"] if name == "track" else mats["stone"] if name == "gravel" else p)
    if "frost" in deco:
        _frost(img, rng, deco["frost"], p)
    if "joints" in deco:
        _joints(img, p)
    if "cracks" in deco:
        _cracks(img, rng, deco["cracks"], p, strong=(name == "ice"))
    if "glints" in deco:
        _glints(img, rng, deco["glints"], p, mats["marsh_water"])
    if "ripples" in deco:
        _ripples(img, rng, deco["ripples"], p)
    if "shoals" in deco:
        _shoals(img, rng, deco["shoals"], p, mats["sand"])
    if "waves" in deco:
        count, amp = deco["waves"]
        _waves(img, rng, count, p, amp=amp)
    if "sparkle" in deco:
        _sparkle(img, rng, deco["sparkle"], p)
    if "puddles" in deco:
        _puddles(img, rng, deco["puddles"], p, mats["mud_wet"])
    if "scuffs" in deco:
        _scuffs(img, rng, deco["scuffs"], p)
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
        **({"liquid": True} if name in LIQUIDS else {}),
    })
    return path


# Ground transitions: a higher-priority terrain draws a soft, noisy edge
# over its lower-priority neighbours. Mask bits N=1, E=2, S=4, W=8 say
# which sides of the lower tile touch the higher terrain.
PRIORITY = ("soil", "soil_rich", "gravel", "stone", "stone_smooth", "sand", "marsh", "ice")
EDGE_DEPTH = 30.0   # px @2x
EDGE_WOBBLE = 9.0


def _side_dist(mask):
    """Distance (px) to the nearest side in mask (bits N=1, E=2, S=4, W=8)."""
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
    return dist


def _edge_img(name, mask, depth=None, wobble=None):
    """Variant 0 of name with a soft, noisy alpha falling off away from
    the sides in mask."""
    depth = EDGE_DEPTH if depth is None else depth
    wobble = EDGE_WOBBLE if wobble is None else wobble
    border = 1000 + sum(map(ord, name))
    base = _tile_img(name, 0)
    wob = _noise(6, border + 13, border * 7 + mask) * wobble
    dist = _side_dist(mask)
    a = np.clip((depth + wob - dist) / 6.0, 0.0, 1.0)
    img = base.copy()
    img[..., 3] = a
    return img


def _edge(name, mask, **kw):
    img = _edge_img(name, mask, **kw)
    out = "%s_edge_%02d" % (name, mask)
    path = S.out_path("terrain", out)
    S.write_png(img, path)
    meta.write("terrain", out, {
        "image": "generated/world/terrain/%s.png" % out, "scale": C.SCALE,
        "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0],
        "transition": name, "mask": mask,
    })
    return path


# Shore foam (water.kage fallback): a wavy pale band drawn over a WATER tile
# along the sides that touch land (or, for shallow over deep, the same
# mask the edge uses). Mask bits N=1, E=2, S=4, W=8. Translucent so the
# water shows through; bubbles trail off into the water.
FOAM_DEPTH = 17.0


def _foam_img(mask):
    p = S.pal()[0]["water_foam"]
    border = 7331
    wob = _noise(6, border, border * 3 + mask) * 6.5 + _noise(12, border + 5, border * 5 + mask) * 2.5
    dist = _side_dist(mask)
    band = FOAM_DEPTH + wob
    a = np.clip((band - dist) / 4.0, 0.0, 1.0)               # solid foam hugging the shore
    lace = np.clip((band + 9.0 - dist) / 5.0, 0.0, 1.0) - a   # thinner lacy trailing line
    lace_pat = (_noise(10, border + 9, border * 7 + mask) > 0.05).astype(np.float32)
    rng = np.random.RandomState(900 + mask)
    bub = np.zeros((N, N), np.float32)
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32)
    for _ in range(10):
        cx, cy = rng.uniform(0, N, 2)
        if dist[int(cy) % N, int(cx) % N] < FOAM_DEPTH + 24:
            r = rng.uniform(2.2, 4.0)
            bub = np.maximum(bub, np.clip(1.6 - np.hypot(xx - cx, yy - cy) / r, 0, 1))
    alpha = np.clip(a * 0.88 + lace * lace_pat * 0.55 + bub * 0.6, 0.0, 0.92)
    img = np.empty((N, N, 4), np.float32)
    img[..., :3] = _rgb(p["base"])
    inner = a > 0.5   # lit crest on the outer edge, shadowed lacing deeper in
    img[inner & (dist > band - 6.0), :3] = _rgb(p["shadow"])
    img[inner & (dist < band * 0.55), :3] = _rgb(p["light"])
    img[..., 3] = alpha
    return img


def _foam(mask):
    img = _foam_img(mask)
    out = "shore_foam_edge_%02d" % mask
    path = S.out_path("terrain", out)
    S.write_png(img, path)
    meta.write("terrain", out, {
        "image": "generated/world/terrain/%s.png" % out, "scale": C.SCALE,
        "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0],
        "transition": "shore_foam", "mask": mask, "overlay_on": "water",
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
    out += build_ng1_edges()
    return out


NG1_TILES = ("shallow_water", "deep_water", "mud", "track")


def build_ng1_edges():
    """Mud over soil, shallow water over deep (wider, softer), shore foam."""
    out = []
    for mask in range(1, 16):
        out.append(_edge("mud", mask))
        out.append(_edge("shallow_water", mask, depth=38.0, wobble=11.0))
        out.append(_foam(mask))
    return out
