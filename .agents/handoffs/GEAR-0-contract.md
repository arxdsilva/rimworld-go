# GEAR-0: contract for gear, outfits, options and ailments

Inert: the state hash is unchanged (`bin/benchmark -hash`, 2026-11-06). The owners named here fill it in. Analyses: `GEAR_GAP_ANALYSIS.md`, `OPTIONS_GAP_ANALYSIS.md`, `AILMENTS_GAP_ANALYSIS.md`.

## Commands (`domain/sim/command`, appended)
- `DropGear{Pawn, Mode, Index, Queue}`, `ConsumeFromPack{Pawn, Index}`, `SetApparelForced{Pawn, Index, Enabled}`. They are routed to `game/sense` (`tick.go`), where `Sense.Apply` has an empty case. Owner: GEAR-1.
- `SetApparelPolicy{Pawn, Index}`, `EditApparelPolicy{Index, Mode, …}` (the `ApparelPolicy*` modes), `ClearForcedApparel{Pawn}`. They are not routed yet. Owner: GEAR-3 (`game/outfit`).

## Read models
- `game/gearview`: `Pawn` is carried as `PawnView.Gear` (mass, capacity, overall armour, move %, apparel policy). `Pack` is `PackItemView.Gear` (grams, use), and `World` is `WorldView.Gear` (policies).
- `craft.WornView`: `Grams`, `Cover`, `MovePct`.
- Stubs:
  - `gearFacts` (`body.go`, GEAR-1).
  - `outfitView` and `outfitWorld` (`apparel.go`, GEAR-3).
- Message and letter `Format`/`Args` are not in the contract. OPT-4 adds them when it starts.

## Settings (`platform/config/prefs.go`, version 10)
- New groups: `General`, `Interface`, `Camera` and `Developer`, with defaults and `Normalize` clamps.
- `General.AutoPause` replaces `Gameplay.PauseOnThreat`. That field is now read only to migrate files older than version 10. `TempKelvin` is added.
- Every default keeps today's behaviour. The one exception is `AutoPause` "major" for a fresh install, which pauses on threat letters only.

## Client
- `ebiten/lang`: `Tr`, `English`, `Tf` and `Live`.
- `ports.Texts` (`T`, `Tf`) is served by the root `shellTexts`, which holds a `*lang.Live`.
- `ports.OpenInfoCard{Pawn, ID, Mode, Index, Kind}` is accepted by `shellDispatch.Do` and does nothing yet. Owner: GEAR-2.

## Budget
`animal_view_species.go` moved to `game/animalview` (hash-neutral), freeing a file and 42 lines in `application/game` for the wave. The ceiling stays where it is until integration lowers it.
