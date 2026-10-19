"""Frozen ART Wave 0 contracts. Change only with an ART spec update."""
import math
import os

# --- scale (§3) ---------------------------------------------------------
TILE_PX = 64            # @1x pixels per logical tile (renderer.TileSize)
SCALE = 2               # we export @2x
PPT = TILE_PX * SCALE   # rendered pixels per tile = 128
# 1 tile = 1 Blender metre. Blender +X = game east, Blender +Y = game north.
# An asset's footprint occupies x in [0, w], y in [-h, 0]: its top-left
# (north-west) corner is the Blender origin, matching game tile (x, y).

# --- camera / perspective (§5) ------------------------------------------
# Orthographic, looking straight down. Volume is shown with an oblique
# shear: a point at height z is drawn SHEAR_K * z tiles further north
# (up the screen). Ground footprints stay exactly grid-aligned and square;
# objects show a little of their south (front) face.
SHEAR_K = 0.5

# Category rule (§5): pawns and animals are upright figures leaned back
# towards the viewer by FIGURE_LEAN_DEG about their feet, so faces,
# apparel and facing read at gameplay zoom. Everything else uses the
# plain shear above. Figures are authored at FIGURE_SCALE.
FIGURE_LEAN_DEG = 55.0
FIGURE_SCALE = 0.92

# --- light (§6) ---------------------------------------------------------
# Upper-left key light: from west/north/above. Vector points FROM the
# scene TOWARDS the light, in sheared (screen) space.
LIGHT_DIR = (-0.48, 0.42, 0.77)
# Toon bands over Lambert intensity (0..1): < SHADOW_T -> shadow,
# < LIGHT_T -> base, else light.
SHADOW_T = 0.38
LIGHT_T = 0.86

# --- outlines & contact shadow (§6), in @2x pixels ----------------------
OUTLINE_PX = 4                 # ~2 px at @1x for the outer contour
OUTLINE_DARKEN = 0.32          # outline = neighbouring colour * this
OUTLINE_SATURATE = 1.25        # keep outlines chromatic, not black
SHADOW_OFFSET = (5, 6)         # lower-right, (x, y) in image pixels
SHADOW_BLUR = 3                # box-blur passes
SHADOW_ALPHA = 0.30
SHADOW_RGB = (0.10, 0.09, 0.08)

# --- canvas -------------------------------------------------------------
DEFAULT_MARGIN = 0.125         # tiles of padding left/right/bottom

# --- paths (§19) --------------------------------------------------------
HERE = os.path.dirname(os.path.abspath(__file__))
SOURCE_DIR = os.path.dirname(HERE)                       # assets/source/blender
ASSETS_DIR = os.path.dirname(os.path.dirname(SOURCE_DIR))  # assets/
GENERATED_DIR = os.path.join(ASSETS_DIR, "generated", "world")
METADATA_DIR = os.path.join(ASSETS_DIR, "metadata")
PALETTE_DIR = os.path.join(ASSETS_DIR, "palettes")
NAMESPACE = "core"
METADATA_VERSION = 1


def light_vector():
    x, y, z = LIGHT_DIR
    n = math.sqrt(x * x + y * y + z * z)
    return (x / n, y / n, z / n)
