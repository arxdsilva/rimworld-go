"""Natural rock walls (ART spec §9): connect visually, read as solid, and
stay distinct from constructed walls: darker, lumpy tops, no rims.
Autotile mask bits N=1, E=2, S=4, W=8, like constructed walls."""
from .. import stage as S

ROCK_H = 1.05
EXT = 0.22
BITS = (("N", 1), ("E", 2), ("S", 4), ("W", 8))


def _rock(mask):
    con = {d for d, b in BITS if mask & b}
    x0 = -EXT if "W" in con else 0.03
    x1 = 1 + EXT if "E" in con else 0.97
    y1 = EXT if "N" in con else -0.03
    y0 = -1 - EXT if "S" in con else -0.97
    S.box("body", x0, y0, 0.0, x1, y1, 0.55, "stone_dark", bevel=0.08, segments=2)
    pts = [(0.28, -0.28), (0.72, -0.32), (0.32, -0.72), (0.70, -0.70), (0.52, -0.50)]
    edge = {"N": (0.5, 0.05), "S": (0.5, -1.05), "E": (1.05, -0.5), "W": (-0.05, -0.5)}
    for d in sorted(con):
        pts.append(edge[d])
    for i, (cx, cy) in enumerate(pts):
        S.blob("lump%d" % i, cx, cy, 0.50, 0.34 if i < 5 else 0.40, "stone" if i % 2 == 0 else "stone_dark",
               seed=mask * 17 + i, lumps=0.32, squash=1.0)
    S.blob("crown", 0.5, -0.45, 0.62, 0.30, "stone", seed=mask * 31 + 5, lumps=0.28, squash=0.9)
    return con


def build():
    out = []
    for mask in range(16):
        con = _rock(mask)
        out.append(S.render_asset("rock", "rock_wall_%02d" % mask, zmax=ROCK_H, clip=tuple(con),
                                  zclip=0.80, extra={"autotile": "rock", "mask": mask, "material": "stone"}))
    return out
