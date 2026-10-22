# UI-015 Frame inspector, carcass art, materials alert

## What changed
- **Carcass art fix:** the read model names carcasses "<species> corpse" but the sprite lookup only mapped "animal corpse", so they drew as orange squares. `itemSpriteSpecies` now maps every species; carried carcasses show their species (`PawnView.CarryingAnimal`). A `KillAnimal` dev hook and a regression test through the real `World()` items cover it.
- **Blueprint/frame inspector:** clicking a frame selects it (object frames before buildings, floor frames last). The panel shows:
  - status: reinstall, no stock (with a craft hint such as "sew one at a tailoring bench"), materials on the way, waiting for hauler, being built by X, waiting for builder;
  - materials delivered/needed (plus on the way) and in stock (reserved);
  - work done/total with an in-game-minute estimate;
  - haulers, builder and footprint.
- **Cancel blueprint** (command `CancelFrame`) refunds delivered materials; the Cancel area tool refunds too.
- **Resource feedback:** the carpet card shows stock and where to make one. New alert "Blueprints waiting for materials (N)".

## Verification
- Tests: `animal_species_test.go` (carcass sprites through real `World()` items) plus frame-view tests.
- Gates: `go build ./...`, `go vet ./...`, `go test ./...`.

## Known limitations
- The materials alert fires immediately, not after an in-game hour as planned (follow-up).
