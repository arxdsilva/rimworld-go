"""Weather art (ART spec §16): ground decals and particle textures.

Painted procedurally with numpy (no Blender geometry), colours blended only
from palette entries so the family stays inside the toon bands.

Ground decals are one full 128 px tile and draw on top of the terrain:
- snow_cover_00..05: soft snow, three value bands with a little sparkle.
  00..03 are partial covers that feather away from one tile side (`edge`
  in the metadata names the side that stays covered: north, east, south,
  west); 04 and 05 are full tiles. All wrap on their covered edges.
- snow_drift_00..03: small lit drifts with a soft contact shadow.
- puddle_00..05: irregular puddles, sky reflection band, darker wet rim.
- frost_00..03: faint crystalline overlays, mostly transparent.

Particles are small sprites (footprint [0, 0], origin at their centre):
splash_00..02 (crown splash frames: small, tall, spent; rain streaks
are drawn in code), snowflake_00..03 (soft flakes, small to large).

    make art FAMILIES=weather
"""
import math

import numpy as np

from .. import contracts as C
from .. import meta, palette
from .. import post
from .. import stage as S

N = C.PPT  # 128


# ------------------------------------------------------------ colours
def _mat(name):
    m = S.pal()[0][name]
    return {k: np.array(palette.hex_to_srgb(m[k]), np.float32) for k in ("light", "base", "shadow")}


def _mix(a, b, t):
    return a * (1 - t) + b * t


def _snow_bands():
    """Cool soft snow: blossom lifted toward glass (both palette entries)."""
    bl, gl = _mat("blossom"), _mat("glass")
    return {"light": bl["light"],
            "base": _mix(bl["base"], gl["light"], 0.5),
            "shadow": _mix(bl["shadow"], gl["base"], 0.45)}


# -------------------------------------------------------------- maths
def _smooth(e0, e1, x):
    t = np.clip((x - e0) / (e1 - e0), 0.0, 1.0)
    return t * t * (3 - 2 * t)


def _vnoise(h, w, cells, seed):
    """Value noise on a (cells x cells) lattice that wraps on both axes."""
    rng = np.random.RandomState(seed)
    g = rng.rand(cells, cells).astype(np.float32)
    ys = np.arange(h, dtype=np.float32) / h * cells
    xs = np.arange(w, dtype=np.float32) / w * cells
    y0 = np.floor(ys).astype(int)
    x0 = np.floor(xs).astype(int)
    fy = (ys - y0)[:, None]
    fx = (xs - x0)[None, :]
    fy = fy * fy * (3 - 2 * fy)
    fx = fx * fx * (3 - 2 * fx)
    y1, x1 = (y0 + 1) % cells, (x0 + 1) % cells
    a = g[y0][:, x0] * (1 - fx) + g[y0][:, x1] * fx
    b = g[y1][:, x0] * (1 - fx) + g[y1][:, x1] * fx
    return a * (1 - fy) + b * fy


def _fbm(n, seed, cells=(4, 8, 16), amps=(0.55, 0.3, 0.15)):
    out = np.zeros((n, n), np.float32)
    for k, (c, a) in enumerate(zip(cells, amps)):
        out += a * _vnoise(n, n, c, seed * 7 + k)
    return out


def _grid(n=N):
    yy, xx = np.mgrid[0:n, 0:n].astype(np.float32) + 0.5
    return xx, yy


def _bands(v, cols, t0, t1, soft=0.05):
    """Three value bands with soft steps; v in 0..1 low -> shadow."""
    s = _smooth(t0 - soft, t0 + soft, v)[..., None]
    l = _smooth(t1 - soft, t1 + soft, v)[..., None]
    rgb = _mix(cols["shadow"], cols["base"], s)
    return _mix(rgb, cols["light"], l)


def _sparkle(rgb, mask, seed, count, col):
    """Tiny plus-shaped glints where the snow is lit."""
    rng = np.random.RandomState(seed)
    h, w = mask.shape
    for _ in range(count):
        x, y = rng.randint(4, w - 4), rng.randint(4, h - 4)
        if mask[y, x] < 0.8:
            continue
        r = rng.choice([1, 1, 2])
        rgb[y, x - r:x + r + 1] = _mix(rgb[y, x - r:x + r + 1], col, 0.9)
        rgb[y - r:y + r + 1, x] = _mix(rgb[y - r:y + r + 1, x], col, 0.9)
        rgb[y, x] = col
    return rgb


def _save(name, img, canvas, origin, footprint, extra):
    path = S.out_path("weather", name)
    S.write_png(img, path)
    data = {"image": "generated/world/weather/%s.png" % name, "scale": C.SCALE,
            "canvas": list(canvas), "footprint": list(footprint), "origin_px": list(origin)}
    data.update(extra)
    meta.write("weather", name, data)
    return path


def _tile(name, img, extra):
    return _save(name, img, (N, N), (0, 0), (1, 1), dict(extra, decal=True))


def _particle(name, img, extra):
    h, w = img.shape[:2]
    return _save(name, img, (w, h), (w // 2, h // 2), (0, 0), dict(extra, particle=True))


def _blur(a, passes):
    return post.box_blur(a, passes)


# ---------------------------------------------------------------- snow
EDGES = ("north", "east", "south", "west")


def _snow_cover(i):
    cols = _snow_bands()
    cols = {"light": cols["light"], "base": cols["base"] * 0.5 + cols["light"] * 0.5,
            "shadow": _mix(cols["base"], cols["shadow"], 0.45)}
    h = _fbm(N, 11 + i)
    rgb = _bands(h, cols, 0.36, 0.62, soft=0.05)
    alpha = np.ones((N, N), np.float32)
    lit = (h > 0.4).astype(np.float32)
    extra = {"weather": "snow", "variant": i, "tileable": i >= 4}
    if i < 4:
        edge = EDGES[i]
        xx, yy = _grid()
        d = {"north": yy, "south": N - yy, "west": xx, "east": N - xx}[edge]  # distance from covered side
        wob = (_vnoise(N, N, 4, 90 + i) - 0.5) * 34
        wob2 = (_vnoise(N, N, 8, 95 + i) - 0.5) * 12
        line = N * 0.46 + wob + wob2
        alpha = 1.0 - _smooth(line - 13, line + 13, d)
        # a cooler, deeper rim where the cover thins out
        rim = _smooth(0.0, 1.0, 1.0 - np.abs(alpha - 0.5) * 2.0) * 0.55
        rgb = _mix(rgb, cols["shadow"], rim[..., None] * (alpha[..., None] > 0.05))
        extra["edge"] = edge
    rgb = _sparkle(rgb, alpha * lit, 400 + i, 16, np.array([1, 1, 1], np.float32))
    img = np.dstack([rgb, alpha])
    return _tile("snow_cover_%02d" % i, img, extra)


def _snow_drift(i):
    cols = _snow_bands()
    xx, yy = _grid()
    rng = np.random.RandomState(70 + i)
    hf = np.zeros((N, N), np.float32)
    blobs = [(64, 66, 40, 24, rng.uniform(-0.5, 0.5)), (44 + 12 * i % 5, 72, 22, 14, rng.uniform(-0.4, 0.4)),
             (88, 60, 24, 16, rng.uniform(-0.6, 0.6))][: 2 + i % 2]
    for cx, cy, rx, ry, ang in blobs:
        c, s = math.cos(ang), math.sin(ang)
        u = ((xx - cx) * c + (yy - cy) * s) / rx
        v = (-(xx - cx) * s + (yy - cy) * c) / ry
        hf = np.maximum(hf, np.clip(1 - u * u - v * v, 0, None) ** 0.9)
    hf += 0.05 * _vnoise(N, N, 8, 80 + i) * (hf > 0)
    hs = _blur(hf, 1)
    gy, gx = np.gradient(hs)
    lx, ly = -0.55, -0.45
    shade = 0.62 - (gx * lx + gy * ly) * 5.0 + 0.25 * (hs - 0.4)
    rgb = _bands(np.clip(shade, 0, 1), cols, 0.42, 0.74, soft=0.06)
    alpha = _smooth(0.015, 0.11, hs)
    # soft contact shadow lower right
    sh = post.shift(alpha, 5, 6)
    sh = _blur(sh, 2) * C.SHADOW_ALPHA * (1 - alpha)
    base_rgb = np.array(C.SHADOW_RGB, np.float32)
    out_a = alpha + sh
    out_rgb = (rgb * alpha[..., None] + base_rgb * sh[..., None]) / np.maximum(out_a, 1e-4)[..., None]
    out_rgb = _sparkle(out_rgb, alpha * (shade > 0.55), 500 + i, 7, np.array([1, 1, 1], np.float32))
    img = np.dstack([out_rgb, out_a])
    return _tile("snow_drift_%02d" % i, img, {"weather": "snow_drift", "variant": i})


# --------------------------------------------------------------- water
def _puddle(i):
    gl, bl, soil = _mat("glass"), _mat("blossom"), _mat("soil_rich")
    xx, yy = _grid()
    rng = np.random.RandomState(30 + i)
    R = 36 + 5 * (i % 3) + 3 * (i // 3)
    asp = 1.15 + 0.18 * ((i * 5) % 3)
    dx, dy = (xx - 64 + rng.uniform(-3, 3)) / asp, (yy - 66) * 1.0
    th = np.arctan2(dy, dx)
    r = np.ones_like(th)
    for k, a in zip((2, 3, 4, 5), (0.10, 0.09, 0.06, 0.04)):
        r += a * np.cos(k * th + rng.uniform(0, 6.28))
    q = np.hypot(dx, dy) / (R * r)
    alpha = 0.9 * (1.0 - _smooth(0.90, 1.0, q))
    # interior: sky toward the top, deeper water below, one reflection band
    t = np.clip((yy - 64 + R) / (2 * R), 0, 1)
    water = _mix(gl["base"], gl["shadow"], _smooth(0.15, 0.95, t)[..., None] * 0.8)
    water = _mix(water, np.array([0.50, 0.58, 0.60], np.float32), 0.15)
    band = _smooth(0.0, 1.0, 1 - np.abs(((xx * 0.55 + yy * 0.8) - (64 * 1.35 + 6 * (i % 3) - 6)) / 11.0))
    band *= (q < 0.78)
    water = _mix(water, _mix(gl["light"], bl["light"], 0.6), (band * 0.6)[..., None])
    glint = _smooth(0.82, 1.0, _vnoise(N, N, 16, 60 + i)) * (q < 0.7) * 0.45
    water = _mix(water, bl["light"], glint[..., None])
    # darker wet rim hugging the border
    rim = _smooth(0.68, 0.93, q) * (1 - _smooth(0.96, 1.02, q))
    rim_col = _mix(soil["shadow"], gl["shadow"], 0.35)
    rgb = _mix(water, rim_col, (rim * 0.78)[..., None])
    img = np.dstack([rgb, alpha])
    return _tile("puddle_%02d" % i, img, {"weather": "puddle", "variant": i})


# --------------------------------------------------------------- frost
def _line_cov(cov, ax, ay, bx, by, width):
    x0, x1 = int(max(0, min(ax, bx) - width - 2)), int(min(N, max(ax, bx) + width + 3))
    y0, y1 = int(max(0, min(ay, by) - width - 2)), int(min(N, max(ay, by) + width + 3))
    if x1 <= x0 or y1 <= y0:
        return
    yy, xx = np.mgrid[y0:y1, x0:x1].astype(np.float32) + 0.5
    vx, vy = bx - ax, by - ay
    ln = max(vx * vx + vy * vy, 1e-6)
    t = np.clip(((xx - ax) * vx + (yy - ay) * vy) / ln, 0, 1)
    d = np.hypot(xx - (ax + vx * t), yy - (ay + vy * t))
    c = np.clip(1.0 - (d - width * 0.5) / 1.0, 0, 1)
    cov[y0:y1, x0:x1] = np.maximum(cov[y0:y1, x0:x1], c)


def _grow(cov, rng, x, y, ang, length, width, depth):
    """Fern-like crystal branch: a stem with 60-degree side shoots."""
    step = 7.0
    n = max(1, int(length / step))
    px, py = x, y
    for k in range(n):
        nx, ny = px + math.cos(ang) * step, py + math.sin(ang) * step
        _line_cov(cov, px, py, nx, ny, width)
        if depth > 0 and k > 0:
            sl = length * (0.55 - 0.03 * k)
            if sl > 6:
                for s in (-1, 1):
                    _grow(cov, rng, px, py, ang + s * math.radians(60), sl, max(1.0, width * 0.8), depth - 1)
        px, py = nx, ny
        length_left = length - (k + 1) * step
        if length_left < 3:
            break


def _frost(i):
    gl, bl = _mat("glass"), _mat("blossom")
    rng = np.random.RandomState(900 + i)
    cov = np.zeros((N, N), np.float32)
    seeds = [(0, 0, 45), (N, 0, 135), (N, N, 225), (0, N, 315)]
    for k, (sx, sy, a0) in enumerate(seeds):
        if (k + i) % 4 == 3:
            continue
        for arm in range(2 + (i + k) % 2):
            ang = math.radians(a0 + rng.uniform(-38, 38))
            _grow(cov, rng, sx, sy, ang, rng.uniform(40, 62), 1.7, 1)
    for _ in range(2):  # a stray crystal along an edge
        e = rng.randint(4)
        t = rng.uniform(24, 104)
        sx, sy = [(t, 0), (N, t), (t, N), (0, t)][e]
        a0 = [90, 180, 270, 0][e] + rng.uniform(-25, 25)
        _grow(cov, rng, sx, sy, math.radians(a0), rng.uniform(22, 38), 1.6, 1)
    haze = _blur(cov, 2) * 0.9
    xx, yy = _grid()
    edge = np.minimum(np.minimum(xx, N - xx), np.minimum(yy, N - yy)) / (N / 2)
    veil = (1 - _smooth(0.0, 0.5, edge)) * (0.35 + 0.65 * _vnoise(N, N, 8, 920 + i)) * 0.16
    a = np.clip(cov * 0.34 + haze * 0.16 + veil * 0.8, 0, 0.5)
    core = _mix(gl["light"], bl["light"], 0.7)
    rgb = _mix(gl["light"], core, np.clip(cov * 1.2, 0, 1)[..., None])
    img = np.dstack([rgb, a])
    return _tile("frost_%02d" % i, img, {"weather": "frost", "variant": i})


# ----------------------------------------------------------- particles
def _soft(img, cov, rgb, alpha=1.0):
    a = np.clip(cov, 0, 1) * alpha
    out_a = a + img[..., 3] * (1 - a)
    safe = np.maximum(out_a, 1e-6)[..., None]
    img[..., :3] = (rgb * a[..., None] + img[..., :3] * (img[..., 3] * (1 - a))[..., None]) / safe
    img[..., 3] = out_a


def _splash(i):
    gl, bl = _mat("glass"), _mat("blossom")
    w, h = 24, 16
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    img = np.zeros((h, w, 4), np.float32)
    img[..., :3] = gl["light"]
    cx, cy = w / 2, h - 4.5
    rx = [4.0, 6.5, 10.0][i]
    ry = [1.3, 2.0, 3.2][i]
    d = np.abs(np.hypot((xx - cx) / rx, (yy - cy) / ry) - 1.0) * min(rx, 8) * 0.22
    ring = np.clip(1.0 - (d * 4.2 - 0.35), 0, 1)
    _soft(img, ring, gl["light"], [0.55, 0.5, 0.4][i])
    if i < 2:  # the crown: droplets thrown up and out along arcs
        up = [5.0, 9.0][i]
        for sg in (-1.0, -0.55, 0.55, 1.0):
            for tt in np.linspace(0.0, 1.0, 10):
                px = cx + sg * (rx * 0.55 + rx * 0.75 * tt)
                py = cy - up * math.sin(math.pi * tt) * (1 - 0.35 * abs(sg)) - 0.3
                dd = np.hypot(xx - px, yy - py)
                _soft(img, np.clip(1.0 - dd, 0, 1), gl["light"], 0.45)
            tx = cx + sg * (rx * 1.3 * (0.8 if i == 0 else 1.0))
            ty = cy - up * (0.75 - 0.2 * abs(sg))
            _soft(img, np.clip(1.5 - np.hypot(xx - tx, yy - ty), 0, 1), bl["light"], 0.95)
        _soft(img, np.clip(1.4 - np.hypot(xx - cx, yy - (cy - up * 1.0)), 0, 1), bl["light"], 0.9)
    else:  # spent: loose droplets falling back, wide faint ripple
        for px, py in [(cx - 7, cy - 3), (cx + 6, cy - 4), (cx - 2, cy - 5), (cx + 9, cy - 1.5)]:
            dd = np.hypot(xx - px, yy - py)
            _soft(img, np.clip(1.3 - dd, 0, 1), bl["light"], 0.6)
    return _particle("splash_%02d" % i, img, {"weather": "splash", "frame": i})


def _snowflake(i):
    bl, gl = _mat("blossom"), _mat("glass")
    size = [10, 14, 18, 24][i]
    h = w = size
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    cx = cy = size / 2
    d = np.hypot(xx - cx, yy - cy) / (size / 2)
    img = np.zeros((h, w, 4), np.float32)
    img[..., :3] = bl["light"]
    core = (1 - _smooth(0.15, 0.95, d)) ** 1.3
    _soft(img, core, _mix(bl["light"], np.array([1, 1, 1], np.float32), 0.6), 0.95)
    if i >= 2:  # a faint six-point star on the larger flakes
        ang = np.arctan2(yy - cy, xx - cx)
        star = np.zeros_like(d)
        for k in range(6):
            a = k * math.pi / 3
            dd = np.abs(-(xx - cx) * math.sin(a) + (yy - cy) * math.cos(a))
            along = (xx - cx) * math.cos(a) + (yy - cy) * math.sin(a)
            star = np.maximum(star, np.clip(1.1 - dd, 0, 1) * (np.abs(along) < size * 0.46))
        _soft(img, star, _mix(gl["light"], bl["light"], 0.5), 0.75 if i == 3 else 0.55)
    img[..., 3] *= 0.85
    return _particle("snowflake_%02d" % i, img, {"weather": "snow_particle", "variant": i})


def build():
    out = [_snow_cover(i) for i in range(6)]
    out += [_snow_drift(i) for i in range(4)]
    out += [_puddle(i) for i in range(6)]
    out += [_frost(i) for i in range(4)]
    out += [_splash(i) for i in range(3)]
    out += [_snowflake(i) for i in range(4)]
    return out
