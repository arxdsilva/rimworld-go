# assets

Presentation-only art. Nothing in this tree is authoritative simulation
state. The renderer and other inbound adapters consume these files;
`internal/domain` must not.

## Vendor pack (first playable)

`Free 2D Animated Vector Game Character Sprites/` is a CC0 pack by
Raphael Gonçalves (Rgsdev). License: `License.txt`.

It is a **side-view** compositional character kit (idle/walk/jump/hit/
death plus environment tiles). It is not the long-term top-down art
bible in `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`. The ebiten
adapter uses it so pawns and ground are readable now; original
orthographic colony art still has to be produced under Wave 0/§34.

IDs the renderer understands:

```text
core:pawn/vendor_char_1 .. core:pawn/vendor_char_4
core:pawn/vendor_enemy_1 .. core:pawn/vendor_enemy_4
core:terrain/vendor_ground
core:overlay/vendor_crosshair
```

Appearance is assigned from `EntityID` in the renderer (stable, not
RNG). Animation frame is derived from simulation tick in the adapter
and never written back to the domain.

## Layout

```text
assets/
├── AGENTS.md
├── palettes/
│   └── ui.yaml          # menu/pause chrome tokens (ART §33)
├── Free 2D Animated Vector Game Character Sprites/   # vendor source
└── .gitkeep
```

Canonical `source/` / `generated/` / `metadata/` layout from the art
spec lands when original art production starts. Do not hand-patch
vendor PNGs; treat the pack as immutable source.
