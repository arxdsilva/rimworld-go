"""Deterministic post-render pass (ART spec §6): chromatic outer contour
and lower-right contact shadow, applied in sRGB image space with numpy.

Images are float32 arrays [H, W, 4], row 0 = top, straight alpha.
"""
import numpy as np

from . import contracts as C

_N4 = [(-1, 0), (1, 0), (0, -1), (0, 1)]
_N8 = _N4 + [(-1, -1), (-1, 1), (1, -1), (1, 1)]


def shift(a, dy, dx):
    """Shift without wrap-around (zero fill)."""
    out = np.zeros_like(a)
    h, w = a.shape[:2]
    ys, yd = (slice(0, h - dy), slice(dy, h)) if dy >= 0 else (slice(-dy, h), slice(0, h + dy))
    xs, xd = (slice(0, w - dx), slice(dx, w)) if dx >= 0 else (slice(-dx, w), slice(0, w + dx))
    out[yd, xd] = a[ys, xs]
    return out


def box_blur(a, passes):
    for _ in range(passes):
        acc = a.copy()
        for dy, dx in _N8:
            acc += shift(a, dy, dx)
        a = acc / 9.0
    return a


def outline(img, width=None, darken=None, saturate=None):
    width = C.OUTLINE_PX if width is None else width
    darken = C.OUTLINE_DARKEN if darken is None else darken
    saturate = C.OUTLINE_SATURATE if saturate is None else saturate
    rgb, a = img[..., :3], img[..., 3]
    solid = a > 0.5
    filled = solid.copy()
    col = np.where(solid[..., None], rgb, 0.0)
    # Grow colour outward; alternate 4/8 neighbourhoods for a rounder contour.
    for step in range(width + 1):
        nbrs = _N8 if step % 2 else _N4
        acc = np.zeros_like(col)
        cnt = np.zeros(a.shape, np.float32)
        for dy, dx in nbrs:
            f = shift(filled, dy, dx)
            acc += shift(col, dy, dx) * f[..., None]
            cnt += f
        new = (~filled) & (cnt > 0)
        col[new] = acc[new] / cnt[new][:, None]
        if step < width:
            filled |= new
        else:
            fringe = new
    lum = col.mean(axis=2, keepdims=True)
    line = np.clip((lum + (col - lum) * saturate) * darken, 0.0, 1.0)
    # Soft outer edge: one-pixel fringe at half strength.
    line_a = filled.astype(np.float32) + fringe.astype(np.float32) * 0.45
    out = np.zeros_like(img)
    # sprite over line
    oa = a + line_a * (1.0 - a)
    safe = np.maximum(oa, 1e-6)[..., None]
    out[..., :3] = (rgb * a[..., None] + line * (line_a * (1.0 - a))[..., None]) / safe
    out[..., 3] = np.clip(oa, 0.0, 1.0)
    return out


def drop_shadow(img, offset=None, blur=None, alpha=None, rgb=None):
    offset = C.SHADOW_OFFSET if offset is None else offset
    blur = C.SHADOW_BLUR if blur is None else blur
    alpha = C.SHADOW_ALPHA if alpha is None else alpha
    rgb = C.SHADOW_RGB if rgb is None else rgb
    a = img[..., 3]
    s = box_blur(shift(a, int(offset[1]), int(offset[0])), blur) * alpha
    oa = a + s * (1.0 - a)
    safe = np.maximum(oa, 1e-6)[..., None]
    out = np.zeros_like(img)
    out[..., :3] = (img[..., :3] * a[..., None] + np.array(rgb, np.float32) * (s * (1.0 - a))[..., None]) / safe
    out[..., 3] = oa
    return out


def process(img, do_outline=True, do_shadow=True, **kw):
    if do_outline:
        img = outline(img, **{k: v for k, v in kw.items() if k in ("width", "darken", "saturate")})
    if do_shadow:
        img = drop_shadow(img)
    return img
