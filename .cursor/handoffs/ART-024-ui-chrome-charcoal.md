# ART-024: UI chrome moved to neutral charcoal with a gold accent

## Why
The user asked for the art direction to come very close to RimWorld's,
style only and clean-room. This change is the UI chrome part of that pass
(plan §3.10). The pawn, camera and world re-render follow as ART-025 and
later.

## What changed
- `widgets.go` tokens and `assets/palettes/ui.yaml`, kept in sync by `TestUITokensMatchPaletteYAML`:

  | Token | Value |
  |---|---|
  | panel | `#26292EE0` |
  | panel border | `#5A5D61` |
  | button | `#3C3F43` |
  | hover | `#4B4E52` |
  | selected | `#5E5440` |
  | disabled | `#24262B` |
  | button border | `#66696E` |
  | accent, focus, slider knob | `#D9B45B` |
  | text | `#E8E4DA` |
  | text dim | `#B8B4AA` |
  | text disabled | `#6D6A64` |
  | heading | `#F0D9A0` |
  | fields | `#1F2022` |
  | tooltip | `#222428F5` |
  | scrims | neutral black |
  | menu space / horizon / dust | `#0E0F12` / `#24262A` / `#8C8576` |
  | danger | lightened to `#FFA094` to keep contrast on the lighter panel |

  The other semantic colours are unchanged.
- `ui_kit.go`: the primary button blends 0.6 toward the accent so its label reaches 5.1:1.
- Tests: `TestUITokensStayClearOfReferencePalette` is replaced by three tests:
  - `TestUIPanelsAreNeutralTranslucent`
  - `TestUITextContrast`: text 11.5:1 and dim 7.0:1 on the panel over a dark map
  - `TestUITokensAreNotCopies`: dE ≥ 3 from the reference hexes, so near-copies are still forbidden
- Docs: ART §33 and the UI screens spec palette line.

## Verify
- `go test -race -run 'UI|Token|Widget|Palette|Contrast|Button' ./internal/adapters/inbound/ebiten/`
- Captures of architect, inspector, health, pawns, menu and storyteller compared with the PLAY-018 set.

## Follow-ups
- The icon badge disc (`overlay.py` ~L178) moves to `#303236` with a `#0F1012` edge when the icons are re-rendered in the world re-render phase.
- The menu diorama grades with `ui.space`, so its next `make art-menu` comes out neutral-dark. The user wanted that.
