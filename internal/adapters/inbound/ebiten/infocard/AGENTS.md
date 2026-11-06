# internal/adapters/inbound/ebiten/infocard

## Purpose
The read-only Info window (GEAR-006): one item described in our own words with the numbers that apply. Opens from `ports.OpenInfoCard` (a pawn's worn garment, weapon, spare or pack entry; a map item by ID; or a bare kind name).

## Owns
- `Window` (`New(Shell)`, `Open`, `Opened`): pushes a `uikit.Modal`, so Esc and the Close button are the kit's. No state of its own.
- `Build(Reader, ports.OpenInfoCard) Card`: pure facts from `PawnView`, `WorldView.Items` and domain catalogs (`armor`, `weapon`, `item`, `quality`). `Layout(Card, w)`: pure wrapped lines.
- `words.go`: the per-category prose.

## Depends on
`gfx`, `uikit`, `ports`; `application/game` read models; domain `combat/armor`, `combat/weapon`, `economy/item`, `economy/quality`.

## Must not depend on
Other feature packages, `render/*`, anything in `application/game` beyond read models.

## Tests
`infocard_test.go`: content per category (apparel, weapon, food, drug, resource), missing items, open/replace/close, fit at UI scales 75-200 %. Root: `journey_infocard_test.go`.
