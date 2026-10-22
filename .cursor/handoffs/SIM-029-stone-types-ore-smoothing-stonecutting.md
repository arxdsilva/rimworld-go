# SIM-029 Stone types, ore veins, smoothing and stonecutting

## What changed
- **`internal/domain/world/geology`:** per-cell stone type (sandstone, granite, limestone, slate, marble) and ore (steel, silver, gold).
  - `Generate(seed, terrain)` picks 2–3 stones per map in noise regions, then grows ore veins from cells at least 2 deep inside rock.
  - Its RNG is `randx.New(seed^Salt)`, never the sim RNG.
  - Value noise moved to `pkg/noise`; generation output is unchanged (verified by hash).
- **Terrain:** adds `RockSmooth` (smoothed wall) and `StoneSmooth` (smooth floor). `IsRockWall` covers both kinds of wall.
- **Items:** five chunk kinds, five block kinds, plus Steel, Silver and Gold. Blocks and steel are building materials. Legacy `Stone` loads as granite blocks.
- **Construction:**
  - `StuffStats` gives per-material HP and work. Granite keeps the old stone numbers; the other stones and steel use original numbers.
  - Walls and doors can be built from steel.
  - New `StonecutterTable` bench, with `Cut<stone>` recipes: 1 chunk becomes 10 blocks. The knife takes any stone blocks.
- **Mining:** rock drops one chunk of its stone. An ore cell drops Steel 20, Silver 25 or Gold 8 and takes 1.5× the work. Smoothed walls can be mined.
- **Smoothing:**
  - `DesignateSmooth` marks rock without ore, and bare rough stone.
  - Builders smooth these under Construction work, after roofs.
  - Covers orders, undesignate, cancel, postpone and the action view.
  - Smooth floors count as floored for rooms.
- **Read model:**
  - `WorldView.StoneType`/`Ore` are shared slices rebuilt only on `StructVer`.
  - Also `Smooths`, `StructVer`, and `Steel`/`Silver`/`Gold`.
  - `game.SoilLabel(ground, stone, ore)` gives hover labels; `SoilView.Smoothable` marks smoothable ground.
- **Save:**
  - Adds `Geo`, `Smooths` and `StructVer`.
  - Old saves regenerate their geology from a hash of the terrain.
  - `item.Stone` becomes `GraniteBlocks` in stacks, buildings, frames, pending, queued and followed orders.
- **Dev tooling:** `Game.SetGround` (dev hook), and `stuffByName` with the `"stone"` alias.

## Verification
- `go test -race ./internal/domain/... ./internal/application/... ./pkg/...`, all green.
- Coverage: application 88.6%; geology and noise 100%.
- New tests:
  - `TestSmoothNoFlap` (the no-flap rule)
  - `TestLegacyStoneMigrates`
  - `TestWorldGeologyDeterministic`
  - `TestStonecutterBillCutsChunks`
- `TestStepIsDeterministicForSameSeed` is unchanged.

## Known limitations
- No fog of war, so ore is visible from the start.
- No mountain (thick) roof.
- Chunks stack like any other item; there are no per-kind stack limits.
- Silver and gold only carry value, since there's no trade yet.
- No beauty stat: smoothing helps room impressiveness only through `Floored` and `StoneWalls`.
- On default 48×40 maps silver and gold are rare. They show up on large, rocky maps.
