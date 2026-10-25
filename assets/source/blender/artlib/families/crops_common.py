"""Crop units (PLAY-016, ART spec §9.3): shared contract for the crop
builders in crops_grain.py and crops_leafy.py.

A crop sprite is ONE small plant (a unit), not a tile: the renderer
composes several units per tile (crop_batch.go). No soil mound (tilled
furrows are an overlay decal), no outline, no baked shadow, the muted
two-band `pl_crop_*` palette (assets/palettes/materials_plants.yaml).
Origin is the ground point under the unit's root.

A builder is `fn(stage, v)` that builds geometry for one unit with the
plant_base primitives (`_limb`, `_leaf`, `_ball`, `_cluster`, `_blade`).
"""
from .plant_base import _asset

# stage keys (unchanged) and variants per stage
STAGES = ("sown", "sprout", "young", "mature", "harvestable", "wilted")
VARIANTS = 4
# low crops render with flat normals so the renderer may mirror them
FLAT = {"lettuce", "radish", "potato", "beans", "ashleaf", "healroot"}
UNIT_M = (0.15, 0.15, 0.1, 0.12)


def unit(species, stage, v, fn, name=None):
    """Render one crop unit sprite plant/<species>_<stage>_v<v>."""
    flat = species in FLAT or stage in ("sown", "sprout")
    extra = {"species": species, "growth": stage, "variant": v, "unit": True}
    if flat:
        extra["flat_normal"] = True
    return _asset(lambda: fn(stage, v), name or "%s_%s_v%d" % (species, stage, v), extra, UNIT_M,
                  outline=False, shadow=False)
