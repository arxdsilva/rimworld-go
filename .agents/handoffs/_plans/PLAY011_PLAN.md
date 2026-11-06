# PLAY-011: auto-oriented doors and gates, in-place material swaps, unmovable fences

## Context
You asked for three things after playing:
1. **Doors and fence gates should face the right way by themselves.** Today you have to rotate them (Q/E) or move them.
2. **Fixed pieces stay put, and their material can change in place.** Walls, doors, fences and gates can't be moved. Placing the same piece on the same tile with a *different* material changes its material; the *same* material does nothing.
3. Your decisions:
   - Swaps happen **in place**: a builder converts the existing piece, the room never opens, and nothing walks through during the job.
   - Scope is **structure + floors + furniture**: walls, doors, fences, floors, carpets, sandbags, beds, double beds, hospital beds, tables and chairs.
   - **Fences and gates become unmovable** like walls and doors; furniture stays movable.

### What the code does today (verified)
**Door** (`construction.go:155`)
- It is not `Rotatable`. `placeFrameStuff` forces `Facing = South` for every non-rotatable kind (`placement.go:328-330`).
- The renderer guesses the door's axis from wall links: `draw_art.go:506-523` sets `ew = mask&(wallE|wallW) != 0`, and `doorSprite` (`archtools.go:345`) gives `structure/door_<wood|steel>_<ns|ew>_<open|closed>`. The art convention (`structure.py:90`): "ew: the door spans east–west (walls left/right, passage north–south)".
- The guess goes wrong in three places:
  - Between fences, because doors only link to walls and rock (`linkgrid.go:61-69`). The mask is 0, so it falls back to `ns`.
  - At corners and T-junctions, where any east or west link wins.
  - In shadows: `shadow.go:321` always draws `ew=false`.

**FenceGate** (`construction.go:190`)
- It is `Rotatable` and stores whatever Q/E facing the Architect had (`architect.go:508`, sent with `DesignateBuild`, parsed at `tick.go:94`).
- `gateSprite(facing, open)` (`fence.go:34`) ignores neighbours. A default "south" gate in a north–south fence line looks crosswise.

**Same kind on the same tile**
- `canPlace` returns `ReasonOccupied` ("already built here", `placement.go:175`), so the ghost shows red.
- The PLAY-010 replace machinery covers only a door or gate over a *different* kind:
  - `replaceTarget`, `ReplacesAt`, `replacedStanding`, `replaceWhy`, `unmarkReplaced`, `cancelReplacing` (`placement.go:208-282`);
  - `Frame.Replaces` / `ReplaceMarked`;
  - the `completeFrame` guard (`tick.go:1260`).

**Materials**
- `Building.Stuff` and `Frame.Stuff` are `item.Kind` (`construction.go:510-522`).
- `StuffOK` (`:399`) validates the material, `StuffStats` (`:134`) sets HP and work percent, and `MaxHP(kind, stuff)` gives the hit points.
- On removal, `removeBuilding` (`placement.go:427-466`) refunds `construction.Refund(kind)` of `refundStuff(b)` via `dropStack`, plus half the components.

**Moving**
- `construction.Movable` (`construction.go:334`) is `k != Wall && k != Door && k != Grave && not a floor`, so **fences and gates can still be moved**.
- The UI mirror is `movableKind` (`move_mode.go:38-46`), used by the Move button (`building_inspector.go:388`), `startMove` and captures.

**Materials per kind:**

| Kind | Materials |
|---|---|
| wall | wood, stone, steel |
| door | wood, steel |
| fence | wood, stone |
| floor | wood, stone |
| carpet | carpet kinds |
| sandbags | wood, stone |
| bed, double bed, hospital bed, table, chair | wood, steel |
| gate | wood only, so a "swap" is always same-material and does nothing |

---

## 1. Auto-orientation of doors and gates

### 1.1 Rule (sim: `placement.go`, new helpers)
`autoFacing(k construction.Kind, x, y int, prefer construction.Facing) construction.Facing` applies to Door and FenceGate; every other kind returns `prefer` unchanged.

A neighbour cell counts as **solid** when it holds, built or as a frame:
- a Wall, Door, Fence or FenceGate on the object layer; or
- natural rock (unwalkable rock terrain, so a door cut into a cliff orients too).

It uses `layerBldgAt` / `layerFrameAt` (`indexes.go:319,350`) and `g.terr.At`.

Count `h` = solid cells at E + W, and `v` = solid cells at N + S:

| Case | Result |
|---|---|
| `h > v` | the piece spans east–west: **South** (door art `ew`; gate south "spans east–west in a fence line", `fence.py:10`) |
| `v > h` | the piece spans north–south: **East** (door art `ns`; gate east) |
| `h == v` (free-standing, a cross, or a 2/2 junction) | keep `prefer` |

**Hysteresis:** a tie keeps the current facing, so an existing piece never flips back and forth.

### 1.2 Where it runs
- **At placement:** in `placeFrameStuff` (`placement.go:324`), after the material check and before `canPlaceAt`: `f = g.autoFacing(kind, x, y, f)`. The player's Q/E is only the tie-breaker.
  - Door stops being forced to South there: the South force applies only to non-rotatable kinds *other than* Door, and Door's facing is stored.
  - Door stays non-`Rotatable` in its Def, so footprint code is unchanged (1×1).
- **When a neighbour changes:** `reorientAround(x, y int)` re-evaluates the doors and gates (buildings and frames) on the 4 orthogonal neighbours of (x, y). Each one gets `autoFacing(kind, nx, ny, current)`, and a building or frame is written only when its facing changes. That's O(1) per change, with no scans. It is called from:
  - `completeFrame` (`tick.go:1259`) for the completed footprint;
  - `removeBuilding` (`placement.go:427`);
  - `placeFrameStuff` after a frame is noted, and `cancelFrame` (`frame_view.go`) / `cancelRect` (`tick.go`), because frames count as neighbours;
  - `setTerrain` (`world.go:92`) when rock is mined away or placed.
  - So a door placed *before* its walls turns to fit once the walls go up, and a gate dropped at the end of a fence run follows the run.
  - Facing changes bump `structVer` (already bumped by those callers).
- **Determinism:** neighbours are read in fixed N, E, S, W order, and there is no RNG.

### 1.3 Old saves
- `saveState` gains `DoorsOriented bool` (gob, additive; new saves write `true`).
- When loading a save without it, after buildings and frames are restored, run `autoFacing(Door, x, y, South)` once for every door building and frame. A door in an old save still sits at South, so a tie keeps South, which matches the old renderer's `ns`/`ew` guess wherever it was unambiguous.
- Gates keep their stored (player-chosen) facing.

### 1.4 Renderer (`internal/adapters/inbound/ebiten`)
- **Door axis comes from the stored facing:** South/North → `ew`, East/West → `ns`. This applies to buildings (`drawBuildingObj`, `draw_art.go:506-523`), frames (`draw_art.go:572`) and the placement ghost (`draw_art.go:200`). The `links.mask` guess for doors is deleted.
- **Shadows:** `shadow.go:321` uses the same axis instead of the hardcoded `ew=false`.
- **Ghost (per-cell facing):**
  - The new read model is `AutoFacingAt(kind, facing string, x, y int) string`, the facing the sim would store.
  - `ghostCell` (`architect.go:558`) gains `facing string`, filled in `previewCells` (`architect.go:569-598`).
  - `drawPlacementGhost` (`draw_art.go:160`) draws each cell with its own facing instead of one facing for all.
  - While dragging a wall line with a door in it, each ghost door previews along the wall run.
- **Hints:**
  - The gate description (`archtools.go:86`) and the Architect hint (`architect.go:470`) lose "Q/E rotates".
  - For doors and gates the hint reads "faces along the wall or fence; Q/E sets a free-standing one".
  - Other rotatable kinds keep Q/E.

---

## 2. In-place material swap

### 2.1 Placement rule (sim: `placement.go`)
`canPlace(k, x, y)` gains the material: `canPlace(k, stuff, x, y)`. `canPlaceAt(k, f, x, y)` becomes `canPlaceAt(k, stuff, f, x, y)`; `CanPlaceAt`, `CanPlace` and `BuildNow` already have the stuff and pass it down.

New `swapTarget(k, stuff, f, x, y) entity.EntityID` returns the existing building when **all** of these hold:
- the building on that layer at the placement origin has the same `Kind`;
- it has the same origin X,Y and, for rotatable multi-tile kinds, the same `Facing`;
- every footprint cell's building on that layer is that same building;
- `b.Stuff != stuff` and `StuffOK(k, stuff)`;
- no frame is already planned on those cells.

Doors and gates compare origin only, because their facing is automatic.

Results:

| Case | Result |
|---|---|
| Swap target found | the footprint cells skip `ReasonOccupied` and `ReasonBlocked`; every other check (bounds, scenario, research, material) still applies |
| Same kind, same origin, same stuff | new **`ReasonSame = "already that material"`**, a neutral refusal |
| Anything else (partial overlap, different footprint or facing, a frame already there) | unchanged (`ReasonOccupied` / `ReasonPlanned`) |

The door-over-wall `replaceTarget` path is unchanged. A door placed over a *door* of another material is now a swap; a door over a wall or fence is still a replacement.

### 2.2 The swap frame
- `construction.Frame` gains `Swaps entity.EntityID` (gob, additive).
- `placeFrameStuff` sets it from `swapTarget`, and does **not** set a deconstruct mark.
- `Need` is the normal material cost of the kind, delivered as the new stuff. **`CompNeed = 0`:** the components already in a hospital bed or tool cabinet are kept.
- `WorkLeft` is `WorkFor(kind, newStuff)`.
- The frame shares cells with its building through the separate frame index; this is already supported, as door-over-wall shows.
  - Walkability is untouched: a wall stays a wall the whole time.
  - `blockingFrameAt` ignores the frame, because the cell is already blocked by the building.
- **Building:** `constructTry` (`tick.go:857`) treats a swap frame like any ready frame. It does *not* wait, unlike `Replaces`. The builder works from the usual work spot (the frame cell is unwalkable for walls, so the existing adjacent-cell rule applies).

### 2.3 Finishing a swap (`completeFrame`, `tick.go:1259`)
When `fr.Swaps` still exists:
1. Delete the frame and update `frameIDs` and indexes, as today.
2. Keep the **same building id**. Set `b.Stuff = fr.Stuff` and `b.Damage = 0`, so HP becomes the new `MaxHP(kind, stuff)`.
   - Facing, claimants, ForPrisoners, bills, fuel, banked work and items on it stay as they are. The door open state is occupancy-driven, so it is unaffected.
3. Refund the old material as a deconstruct would: `construction.Refund(kind)` of `refundStuff(old)` via `dropStack` at the building. No components are refunded, because they stay in the building.
4. Run `markRoomsDirty()` and `structVer++`: stone walls change room impressiveness and beauty, and the stone tint changes.
5. Log nothing; a message appears only if the old refund can't be dropped.

If the target is gone (deconstructed, destroyed, or moved away before completion), the frame completes as a **normal new building**, which is today's path.

### 2.4 Guards and edge cases
- **Move:** moving a building with a pending swap frame is refused with **`ReasonSwapPending = "Material change in progress"`** (`canMoveTo`, `move.go:39`). The Move button shows that reason.
- **Deconstructing the target:** the frame stays and becomes a normal build when it completes.
- **Cancelling the swap frame** (cancel tool, frame inspector): it refunds delivered material as any frame cancel does, and the building is untouched.
- **Damaged buildings:** a swap also repairs, because damage resets at completion. The repair job (`defense.go`) skips buildings with a pending swap frame, so two builders don't work the same tile.
- **Beds:** owners keep their claim through the swap. A bed being swapped stays usable until it completes, and sleeping pawns are not ejected.
- **Saves:** `Frame.Swaps` rides along in the frame gob. A round-trip test proves it.

### 2.5 Read models and UI
- **`ReplacesAt(kind, stuff string, x, y int) string`** gains `stuff`. It returns:
  - door-over-wall: `"replaces wall"`, as today;
  - a swap: `"wood wall → steel"`, using the existing material label helper for `Stuff` names;
  - otherwise `""`.
- **Architect** (`architect.go`):
  - `previewCells` marks `ReasonSame` cells with a new `same bool`. They draw in a neutral grey, not red, and give no "can't place" text.
  - Swap cells are valid (green) and show the swap label next to the first one.
  - `submitArchitect` (`architect.go:614`) silently skips same-material cells, so dragging a steel wall over a run that is half steel only sends the wood half.
  - The ghost's count/cost total counts only swap and new cells.
- **Frame inspector** (`frame_view.go` `frameStatus`): "Changing to steel" while the swap is pending.
- **Building inspector** (`building_inspector.go`): a line "Changing to steel (N%)". It comes from new `BuildingView.SwapTo string` and `SwapPct int`, filled by the sim when a swap frame targets the building; the percentage is the work done.

### 2.6 Unmovable fences and gates
- `construction.Movable` (`construction.go:334`) also excludes `Fence` and `FenceGate`.
- `movableKind` (`move_mode.go:38-46`) excludes "fence" and "fence gate".
- The Move button is disabled with the existing "Can't be moved" reason.
- Fences and gates already placed in saves simply lose the Move option.

---

## 3. Execution (parallel-work rules: two areas, contract first)

### 3.1 Lead, Phase 0 (one commit, before agents)
- Add the constants `ReasonSame` and `ReasonSwapPending` (`placement.go` reason block / `move.go`).
- Add `construction.Frame.Swaps entity.EntityID` and `saveState.DoorsOriented bool`.
- Add `BuildingView.SwapTo string`, `SwapPct int` (`game.go`).
- Declare the read-model methods on `*game.Game`:
  - `AutoFacingAt(kind, facing string, x, y int) string`, which for now returns `facing`;
  - `ReplacesAt(kind, stuff string, x, y int) string`, the new signature.
- Add them to the renderer's `Simulation` interface (`renderer.go:41-44`) and to the `stubSim` in `app_test.go`, then build green.

### 3.2 Agents (run in parallel)
| Agent | Model (why) | Owns | Must not touch |
|---|---|---|---|
| **SIM** | opus (placement rules, frame lifecycle, saves, hysteresis) | `game/placement.go` (`autoFacing`, `reorientAround`, `swapTarget`, `canPlace`/`canPlaceAt`/`placeFrameStuff`, `ReplacesAt`, `AutoFacingAt`), `game/tick.go` (`completeFrame` swap path and reorient calls, `constructTry` swap acceptance, `cancelRect` reorient), `game/frame_view.go` (status, cancel reorient), `game/move.go` (`ReasonSwapPending`), `game/defense.go` (repair skip), `game/world.go` `setTerrain` reorient, `game/save.go` (`DoorsOriented` + migration), the BuildingView swap fill, `domain/building/construction` (`Movable`), sim tests | `ebiten/*` |
| **RENDER** | sonnet (renderer and UI against a written contract) | `ebiten/draw_art.go` (door axis from facing for building, frame and ghost; per-cell ghost facing), `ebiten/shadow.go` (door axis), `ebiten/architect.go` (ghostCell facing and same, neutral same cells, swap label, submit skip, hints), `ebiten/archtools.go` (descriptions), `ebiten/move_mode.go` (`movableKind`), `ebiten/building_inspector.go` (swap line), renderer tests | `internal/application/**` |

**Both:** TDD (red first); targeted `go test -race -cover` on their packages; coverage ≥ 80% or the gap reported; reports of 200 words or fewer; no commits.

### 3.3 Tests (red first)
**SIM (`internal/application/game`, new `orient_test.go`, `swap_test.go`):**
- **Orientation:**
  - A door in a horizontal wall run faces South, and in a vertical run East.
  - A door between two fences faces along the fence.
  - A door at a T-junction (3 walls) picks the longer run (2 vs 1).
  - A free-standing door keeps the player's facing (tie).
  - A door placed first turns when its walls are built, both for a frame and for a built door.
  - Removing one wall of a 2/2 cross doesn't flip a door, but removing both of one side does.
  - **No-flap:** alternately building and removing a single neighbour on a tie cell changes facing at most once per real majority change.
  - A gate in a vertical fence line faces East.
  - A door in a rock cliff orients.
  - Loading an old save (no `DoorsOriented`) orients its doors; gates keep their facing.
- **Swap:**
  - A wood wall painted steel creates one frame with `Swaps` set and no deconstruct mark. The cell is never walkable during the job. When done, it is the same building id with steel stuff, full steel HP, a wood refund on the ground, and rooms re-scored.
  - A steel door over a wood door: the door keeps working while being changed.
  - A bed swap keeps its claimant; a hospital bed swap needs no components.
  - A wood floor to stone; a carpet colour change.
  - A table (multi-tile): only an identical footprint and facing swaps; an offset placement is Occupied.
  - The same material gives `ReasonSame`, with no frame and no message.
  - A gate swap is always `ReasonSame` (wood only).
  - Moving a building with a pending swap gives `ReasonSwapPending`.
  - Deconstructing the target turns the frame into a normal new build.
  - Cancelling the swap frame refunds and leaves the building untouched.
  - The repair job skips a building with a swap pending.
  - A save round-trip keeps `Swaps`.
- **Moving:** extend `TestCannotMoveWallOrGrave` (`move_test.go:196`) to fence and gate.
- **Assertions updated on purpose:**
  - `TestReplaceCanPlaceRules` (`door_replace_test.go:38-55`): wall-on-wall and door-on-door with a *different* material is now a swap, and the same material is now `ReasonSame` (was `ReasonOccupied`).
  - `TestCanPlaceReasons` (`orders_test.go:27`): same-kind cases.

**RENDER (`internal/adapters/inbound/ebiten`):**
- The door sprite axis follows the stored facing for buildings, frames and the ghost.
- The shadow uses the door axis.
- The ghost draws each cell with its own facing (stub `AutoFacingAt`).
- `ReasonSame` cells are neutral and not submitted.
- The swap label reads "wood wall → steel".
- The building inspector shows "Changing to steel (N%)".
- The Move button is disabled for fences and gates (`TestMoveButtonDisabledForWalls`, `move_mode_test.go:61`).
- Updated to the facing source: `TestFenceSpriteKeys` (`frame_ui_test.go:255`), the door key test in `stone_art_test.go:63`, and `TestGhostAndFrameWallsDrawWithArt` (`linkgrid_test.go:411`).

---

## 4. Verification (lead, after both agents)
1. `go build ./...`, `gofmt -l` (empty), `go vet ./...`, `staticcheck ./...` (no new findings).
2. `go test -count=1 -timeout 40m ./...`; `-race` on `internal/application/game` and `internal/adapters/inbound/ebiten`.
3. **Determinism and goldens:**
   - Determinism ×3 (`-run 'Golden|Determin|SameSeed|Scale'`).
   - Goldens should hold, since legacy colonies build no doors or gates and nothing is swapped. If they move, re-pin only after a field-by-field diff.
4. `make art-check`; no new art.
5. **Manual run** (`go run ./cmd/game`):
   - Draw a walled box and drop a door into each side: each door faces along its wall.
   - Drop a gate into a north–south fence line: it faces along the line.
   - Place a door first, then the walls: it turns to fit.
   - Paint steel walls over a wood room: no gap appears, the room stays sealed, the walls turn steel one by one, and wood is refunded.
   - Paint wood over wood: grey ghost, nothing queued.
   - Select a fence: Move is disabled.
   - Swap a bed's material: the owner keeps it.
6. **Docs:**
   - Handoff `.cursor/handoffs/UI-022-orientation-and-material-swap.md`.
   - Vanilla checklist: Blueprint (auto facing, swap) and Deconstruction/Replace items.
   - UI/UX checklist: Ghost blueprint (per-cell facing, swap label) and Validity visualization (neutral same-material).
   - README construction line ("doors and gates face along their walls; paint a different material over a wall, floor or furniture to change it in place").
6. Commits on `main` (contract, then sim, then renderer + docs); no push.

## Critical files
- `internal/application/game/placement.go` (`autoFacing`, `reorientAround`, `swapTarget`, `canPlace`, `placeFrameStuff`, `ReplacesAt`, `AutoFacingAt`)
- `internal/application/game/tick.go` (`completeFrame`, `constructTry`, `cancelRect`)
- `internal/application/game/{frame_view,move,defense,world,save,game}.go`
- `internal/domain/building/construction/construction.go` (`Frame.Swaps`, `Movable`)
- `internal/adapters/inbound/ebiten/{draw_art,shadow,architect,archtools,move_mode,building_inspector,renderer}.go`
