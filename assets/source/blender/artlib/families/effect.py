"""Transient effects (ART spec §16, NG-1 §7.3): the drop pod's descent
streak and the impact dust puff.

Painted procedurally with numpy (like weather particles); colours blend
only palette entries. Both are particles (footprint [0, 0], origin at the
anchor point) that the renderer scales, rotates and fades on the
presentation clock; animation never feeds the simulation.

- pod_fall: 96x448. A vertical streak, tail at the top and the glowing
  head at the bottom, so drawn with its origin on the impact tile the pod
  seems to fall straight down. White-hot core, amber flame, grey-brown
  smoke, a few sparks. Origin = the head centre.
- pod_dust: 256x192. A low, flat ring of dust puffs thrown outward on
  impact (three light/base/shadow bands, soft edge). Origin = its centre.

    make art FAMILIES=effect
"""
import math

import numpy as np

from .. import contracts as C
from .. import meta, palette
from .. import stage as S

N = C.PPT


def _mat(name):
    m = S.pal()[0][name]
    return {k: np.array(palette.hex_to_srgb(m[k]), np.float32) for k in ("light", "base", "shadow")}


def _mix(a, b, t):
    return a * (1 - t) + b * t


def _smooth(e0, e1, x):
    t = np.clip((x - e0) / (e1 - e0), 0.0, 1.0)
    return t * t * (3 - 2 * t)


def _over(img, cov, rgb, alpha=1.0):
    a = np.clip(cov, 0, 1) * alpha
    out_a = a + img[..., 3] * (1 - a)
    safe = np.maximum(out_a, 1e-6)[..., None]
    img[..., :3] = (np.asarray(rgb, np.float32) * a[..., None] + img[..., :3] * (img[..., 3] * (1 - a))[..., None]) / safe
    img[..., 3] = out_a


def _save(name, img, origin, extra):
    h, w = img.shape[:2]
    path = S.out_path("effect", name)
    S.write_png(img, path)
    data = {"image": "generated/world/effect/%s.png" % name, "scale": C.SCALE, "canvas": [w, h],
            "footprint": [0, 0], "origin_px": list(origin), "particle": True,
            "shadow": {"kind": "none", "w": 0.0, "h": 0.0, "d": 0.0}}
    data.update(extra)
    meta.write("effect", name, data)
    return path


def _pod_fall():
    glow, paint, smoke, dust = _mat("pod_glow"), _mat("pod_paint"), _mat("pod_scorch"), _mat("dust_cloud")
    w, h = 96, 448
    head = (w / 2, 392.0)
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    rng = np.random.RandomState(4242)
    img = np.zeros((h, w, 4), np.float32)
    img[..., :3] = glow["base"]
    t = np.clip((head[1] - yy) / head[1], 0, 1)          # 0 at the head .. 1 at the tail
    wob = np.sin(yy * 0.045) * 2.5 * t                   # the tail drifts a little
    dx = np.abs(xx - head[0] - wob)
    # smoke: widest and palest at the tail, grey-brown
    half_s = 9.0 + 20.0 * t ** 0.8
    cov_s = np.clip(1.0 - (dx - half_s) / 5.0, 0, 1) * (yy < head[1] + 10) * (1 - _smooth(0.55, 1.0, t)) * 0.7
    _over(img, cov_s, _mix(smoke["base"], dust["base"], 0.35 + 0.4 * t[..., None]), 1.0)
    # flame: amber, tapering into the tail
    half_f = 4.0 + 17.0 * (1 - t) ** 0.35 * (0.2 + 0.8 * _smooth(0.0, 0.18, t))
    cov_f = np.clip(1.0 - (dx - half_f * (1 - 0.55 * t)) / 3.5, 0, 1) * (yy < head[1] + 6) * (1 - _smooth(0.4, 0.9, t))
    _over(img, cov_f, _mix(paint["light"], glow["base"], (1 - t)[..., None] * 0.9), 0.95)
    # white-hot core
    half_c = 3.0 + 9.0 * (1 - t) ** 0.6
    cov_c = np.clip(1.0 - (dx - half_c) / 2.5, 0, 1) * (yy < head[1] + 4) * (1 - _smooth(0.0, 0.5, t))
    _over(img, cov_c, _mix(glow["light"], np.ones(3, np.float32), 0.55), 1.0)
    # the pod's glowing nose
    nose = np.hypot((xx - head[0]) / 22.0, (yy - head[1]) / 26.0)
    _over(img, np.clip(1.15 - nose, 0, 1) * 3.0, _mix(paint["base"], glow["base"], 0.35), 1.0)
    _over(img, np.clip(0.78 - nose, 0, 1) * 3.0, glow["light"], 1.0)
    _over(img, np.clip(0.36 - np.hypot((xx - head[0] + 5) / 14.0, (yy - head[1] + 6) / 16.0), 0, 1) * 4.0,
          np.ones(3, np.float32), 1.0)
    # sparks shed along the flame
    for _ in range(14):
        sy = rng.uniform(40, head[1] - 20)
        sx = head[0] + rng.uniform(-1, 1) * (12 + 22 * (1 - sy / head[1]))
        r = rng.uniform(1.4, 2.6)
        _over(img, np.clip(1.4 - np.hypot(xx - sx, yy - sy) / r, 0, 1), glow["light"], 0.9)
    return _save("pod_fall", img, (int(head[0]), int(head[1])), {"effect": "pod_fall"})


def _pod_dust():
    cl = _mat("dust_cloud")
    w, h = 256, 192
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    cx, cy = w / 2, h / 2
    rng = np.random.RandomState(515)
    img = np.zeros((h, w, 4), np.float32)
    img[..., :3] = cl["base"]
    puffs = []
    for k in range(14):                                 # a ragged ring of overlapping puffs
        a = 2 * math.pi * (k + rng.uniform(-0.25, 0.25)) / 14
        rad = rng.uniform(0.62, 0.8)
        r = rng.uniform(15, 26)
        puffs.append((cx + math.cos(a) * rad * 100, cy + math.sin(a) * rad * 74, r))
    for _ in range(8):                                  # plus an inner haze
        a = rng.uniform(0, 2 * math.pi)
        rad = rng.uniform(0.0, 0.45)
        puffs.append((cx + math.cos(a) * rad * 100, cy + math.sin(a) * rad * 74, rng.uniform(18, 28)))
    cov = np.zeros((h, w), np.float32)
    z = np.zeros((h, w), np.float32)
    for px, py, r in puffs:                             # each puff is a soft dome
        d = np.hypot((xx - px) / 1.2, yy - py) / r
        cov = np.maximum(cov, np.clip(1.25 - d, 0, 1))
        z = np.maximum(z, np.sqrt(np.clip(1.0 - d * d, 0, 1)) * r)
    zs = np.roll(np.roll(z, 4, axis=0), 4, axis=1)      # lit where the dome rises toward the upper left
    slope = z - zs
    rgb = np.empty((h, w, 3), np.float32)
    rgb[:] = cl["base"]
    rgb[slope > 2.6] = cl["light"]
    rgb[slope < -2.6] = cl["shadow"]
    img[..., :3] = rgb
    img[..., 3] = _smooth(0.05, 0.7, cov) * 0.82
    return _save("pod_dust", img, (int(cx), int(cy)), {"effect": "pod_dust"})


def _steam_puff(k):
    """Frame k (0..3) of a soft rising steam plume, 128x192, origin at the base
    centre (the vent). White-grey, semi-transparent, no outline. Frame 0 is a
    low wisp, 1-2 the plume at full height, 3 a thinning, widening dissipation."""
    w, h = 128, 192
    cl = _mat("dust_cloud")
    white = np.array(palette.hex_to_srgb(S.pal()[0]["med_white"]["base"]), np.float32)
    grey = np.array(palette.hex_to_srgb(S.pal()[0]["med_white"]["shadow"]), np.float32)
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    base = (w / 2, h - 10.0)
    height = (60, 120, 160, 176)[k]
    spread = (0.7, 1.0, 1.3, 1.7)[k]
    fade = (0.75, 0.62, 0.48, 0.28)[k]
    rng = np.random.RandomState(777)
    cov = np.zeros((h, w), np.float32)
    for i in range(14):
        t = (i + rng.uniform(0, 0.8)) / 14.0
        py = base[1] - t * height
        px = base[0] + np.sin(t * 5.0 + 1.3) * 9 * t + rng.uniform(-5, 5) * t
        r = (9 + 18 * t) * spread * rng.uniform(0.85, 1.15)
        d = np.hypot(xx - px, (yy - py) * 1.1) / r
        cov = np.maximum(cov, np.clip(1.2 - d, 0, 1) * (1.0 - 0.8 * t ** 1.5))
    shade = _smooth(0.0, 1.0, (yy - (base[1] - height)) / max(height, 1))   # grey low and inside, white high
    rgb = np.empty((h, w, 3), np.float32)
    rgb[:] = _mix(white, grey, 0.35 * shade[..., None])
    rgb = _mix(rgb, cl["light"], 0.12)
    img = np.zeros((h, w, 4), np.float32)
    img[..., :3] = rgb
    img[..., 3] = _smooth(0.05, 0.8, cov) * fade
    return _save("steam_puff_%d" % k, img, (int(base[0]), int(base[1])), {"effect": "steam_puff", "frame": k})


def build():
    return [_pod_fall(), _pod_dust()] + [_steam_puff(k) for k in range(4)]
