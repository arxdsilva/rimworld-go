# PWR: visible power wires and Reconnect

Asked 2026-11-03: powered items such as wall lamps, coolers and heaters should connect with a small visible wire to the closest power source or wiring, even when it is built in a wall, and the game should connect and disconnect the way RimWorld does. Detail, wiki sources and rows PWR-1 to PWR-5: `.cursor/POWER_GAP_ANALYSIS.md`. Plan: `~/.claude/plans/redraw-the-narrators-they-glimmering-lampson.md`.

## What changed
- Graph (`domain/building/power`): `ConnectRange` 3 to 6 (Chebyshev, ours). Generators and batteries conduct. A conductor touching another conductor is on that grid with no wire. A set of only generators and batteries wires out once. `Node.Pin` is the conductor the player chose; `Link.Next` is the conductor Reconnect moves to. Of conductors equally far the wire takes the straightest, then the smaller y, then the smaller x (`footDist`), so a lamp above a conduit doesn't wire to a diagonal tile. `Role.Device`, `Spec.Thermostat`, `Mix`, `FlagBits`, `SortByNode` moved in from `application/game` (`listing.go`) to pay the `game` budget.
- Simulation (`application/game`): `command.ReconnectPower` (`cmdReconnectPower` in `power_flick.go`), `powerState.pins` (pruned in `prunePower`), `powerSave.Pins` (additive), `BuildingView.PowerRelink`, `powerCache.viewVer` now hashes the wire.
- Wire (`ebiten/render`): `overlay/power_cords.go`. `Wiring.DrawCords` is called in `world/draw_art.go` after the object pass, before weather and fog, from the device cell's edge; `Wiring.DrawCordStubs` runs just before the objects, from the cell's centre to that edge, so a sprite of any size hides what it covers and the wire still reaches it. `Bakes.CordChunks` in `world/lod.go`. Tokens `ui.power_cord`, `ui.power_cord_rim` (`assets/palettes/ui.yaml`, `uikit`). The rim exists because the dark line alone vanished on a slate floor in a capture.
- Overlay and ghost: `PowerOverlay.GhostWire`, `Transmitters` includes generators and batteries, the red "not connected" frame keys on `PowerNet == 0`. Placed power blueprints draw a dimmer wire (PWR-5).
- UI: Reconnect button `cmd.power.reconnect` in `inspector/cmds_power.go` with three disabled reasons; icon `power_reconnect` (`overlay.py`, `power_sheet.py`, `assets/metadata/ui_icon/power_reconnect.yaml`); catalog cards, legend and overlay tip say 6 tiles and name generators and batteries.

## Files
- Domain: `internal/domain/building/power/{graph.go,power.go,listing.go}` and tests.
- Game: `internal/application/game/{power.go,power_flick.go,power_view.go,host_commands.go,tick.go}`, `power_reconnect_test.go`, `power_sim_test.go`, `power_contract_test.go`.
- Ebiten: `render/overlay/{power_cords.go,power_overlay.go,power_draw.go,set.go}`, `render/world/{draw_art.go,lod.go}`, `architect/{catalog.go,marks.go}`, `inspector/cmds_power.go`, `journey_power_wire_test.go`.
- Art: `assets/source/blender/artlib/families/overlay.py`, `assets/source/tools/power_sheet.py`, generated icon and metadata.
- Docs: `POWER_GAP_ANALYSIS.md`, this file, the checklist rows, `DOCUMENT_INDEX.md`, the overlay and inspector `AGENTS.md`.

## Tests
- Graph: `TestBuildConnectRange`, `TestBuildConductors`, `TestBuildPin`, `TestBuildPinCarriesWiredSets`, `TestBuildNextCycles`, `TestBuildLaws`, `TestBuildOneWirePerDeviceSet`. `TestBuildSwitchCuts`, the tie cases of `TestBuildNextCycles` and one case of `TestBuildPin` moved to the straightest-first rule on purpose.
- Simulation: `TestReconnectMovesToTheOtherGrid`, `TestReconnectRefused`, `TestReconnectNoFlap`, `TestReconnectThroughASwitch`, `TestReconnectPinSurvivesSaveAndLoad`, `TestReconnectPinPruned`, `TestPowerSaveWithNoPins`, `TestReconnectBattery`.
- Drawing: `power_cords_test.go` (wires, none without a real wire, not on the Map tier, view cut, blueprints, zero allocations, `TestGhostWireAgreesWithBuild`, `TestCordStartsAtTheCellEdge`, `TestCordStubJoinsTheDevice`), `render/world/cords_test.go` (draw order, chunks widened, map edge).
- Journey: `TestJourneyPowerReconnect`.
- Old range tests moved from 3 to 6 with the same boundary checks: `TestBuildConnectRange`, `power_ui_test.go`, `power_sim_test.go`.

## Decisions
- Wiki numbers for power (6 tiles, what carries power) by the user's approval of the plan, 2026-11-03. The user then added power to the `AGENTS.md` ENG-003 line the same day.
- No Disconnect button, as the wiki names none.
- Reconnect cycles by grid, applies at once, and is disabled with a reason when there is nothing to move to.
- The wire is our own colour; the overlay colours it by grid.
- Old saves connect more (range 6, conducting devices). Links are derived, so no migration.

## Residual
- Ghost known gap: a ghost touching a wireless generator and battery grid, with a conductor in its own range, shows no wire where the built device gets one.
- Two device-only sets in range each wire once, so two wires may overlap.
- A pin whose target is gone stays stored and falls back to nearest; a rebuilt conduit has a new id, so the wire does not jump back.
- A wire passes over a pawn walking beneath it.
- A placed blueprint's wire has no stub, so it starts at the cell's edge. A wire passes over the part of a sprite that overhangs its own cell northward, such as the top of a stove.

## Verification
Run 2026-11-03 on the final tree, by the lead.
- [x] Build, gofmt, `make vet`, `go test -count=1 ./internal/arch/`: pass. `make lint` can't run in this environment.
- [x] `go test -race -count=1 -p 2 -timeout 10m ./...`: 177 packages ok, none failed. The first run caught an inline comment in `ui.yaml` that the contract commit added (`TestUITokensMatchPaletteYAML`); fixed here.
- [x] `go test -count=2 -run 'Determin|SameSeed|Hash|Golden' ./internal/...` (no `-race`): pass.
- [x] `make art-check`: pass. 62 other `assets/generated/ui/icons/*.png` were rewritten with noise by the overlay family build and are not in this change.
- [x] `TestUIDrawsThroughCanvas`, `TestBuildingViewEqual`, `TestEveryInputActionHasAJourney`: pass, in the full run.
- [x] `BenchmarkTick*`, three alternating runs against 81a280b75: Grown/120 +2.4 %, Grown/500 0.0 %, Raid/500 +1.5 %. Grown/250 by median -3.3 %; one old run had a 285 us outlier.
- [x] DrawArt500/z1 A/B, three alternating runs: +0.8 % ns/op, 400 draw calls both, allocations 3442 to 3445 both. That scene has no wired device, so it doesn't exercise the wire passes; a frame with wires costs two more draw calls (stubs and wires), not the one the plan named.
- [x] `TestGrownGolden`: no re-pin. The 250x250 grown colony has 6 lamps, no conductors and no nets, so these rules don't reach it.
- [x] Budget (`solid_budget.txt`): `lines game` lowered from 53284 to 53271; nothing raised.
- [x] Capture checked by eye (`-scenario power`): wall lamp, heater, cooler in a wall and the conduit under a wall, battery, generator; soil, gravel and a slate floor; zooms 0.55, 0.8, 1, 1.5, 2. Not looked at: night, an onyx carpet (`#1e1e1e`, the darkest colour), a placed blueprint's wire (tests only).
- [ ] In play: not hand-played. `TestJourneyPowerReconnect` drives the real click path for Reconnect; placing a lamp and flipping a switch are covered by tests, not by a person.
