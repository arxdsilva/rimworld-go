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
SHEAR_K = 0.35

# Category rule (§5): pawns and animals are upright figures leaned back
# towards the viewer by FIGURE_LEAN_DEG about their feet, so faces,
# apparel and facing read at gameplay zoom. Everything else uses the
# plain shear above. Figures are authored at FIGURE_SCALE.
FIGURE_LEAN_DEG = 82.0
FIGURE_SCALE = 0.92
# Animals lean less than people so the body stays long (animals/rig.py
# and stage._shadow_meta read this; never hard-code it again).
ANIMAL_LEAN_DEG = 70.0

# --- light (§6) ---------------------------------------------------------
# Upper-left key light: from west/north/above. Vector points FROM the
# scene TOWARDS the light, in sheared (screen) space.
LIGHT_DIR = (-0.48, 0.42, 0.77)
# Toon bands over Lambert intensity (0..1): < SHADOW_T -> shadow,
# < LIGHT_T -> base, else light.
SHADOW_T = 0.30
LIGHT_T = 0.80
# Colour-ramp interpolation between the bands: "CONSTANT" gives three
# exact colours; "EASE" or "LINEAR" blend shadow->base and base->light
# over RAMP_SOFT (Lambert units) either side of each threshold, so the
# bands stay flat colour regions with soft edges.
RAMP_INTERP = "EASE"
RAMP_SOFT = 0.06

# --- outlines & contact shadow (§6), in @2x pixels ----------------------
OUTLINE_PX = 6                 # ~2 px at @1x for the outer contour
OUTLINE_DARKEN = 0.18          # outline = neighbouring colour * this
OUTLINE_SATURATE = 1.05        # keep outlines chromatic, not black
OUTLINE_FRINGE = 0.45          # alpha of the one-pixel soft outer fringe
# Layered figures (pawn apparel, hair, hats rendered over a body holdout):
# where a layer's outline would fall on the figure itself (hair onto the
# face, pants onto the torso) it is capped at this width; edges that meet
# transparency keep the full OUTLINE_PX contour.
OUTLINE_INNER_PX = 2
OUTLINE_INNER_FAMILIES = ("pawn",)
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
# Review renders: RWG_OUT_ROOT=<dir> writes generated/ and metadata/ under
# <dir> instead of assets/ (prototypes never touch the committed set);
# RWG_PALETTE_DIR=<dir> reads materials*.yaml / base.yaml from <dir>.
OUT_ROOT = os.environ.get("RWG_OUT_ROOT") or ASSETS_DIR
GENERATED_DIR = os.path.join(OUT_ROOT, "generated", "world")
METADATA_DIR = os.path.join(OUT_ROOT, "metadata")
PALETTE_DIR = os.environ.get("RWG_PALETTE_DIR") or os.path.join(ASSETS_DIR, "palettes")
NAMESPACE = "core"
METADATA_VERSION = 1


def light_vector():
    x, y, z = LIGHT_DIR
    n = math.sqrt(x * x + y * y + z * z)
    return (x / n, y / n, z / n)
