# Task Handoff

Task ID: ENG-003-social-contract (M0 of the social and host systems plan)
Status: done (contract only; no feature behaviour)

## Summary

Implements `.cursor/SOCIAL_SYSTEM_CONTRACT.md` C1-C4: every shared enum and
def field appended, the seven domain packages as compilable stubs, the
application skeleton (phases, containers, RNG streams, hooks at their call
sites, job dispatch, read models, save fields) and the UI contract (Simulation
methods, Health/Social tabs, Misc architect category). Two intended behaviour
changes: `mood.ColonistDied` is -3 for 6 days, and Doctor work now also serves
Doctor-skill bills (Refine industrial medicine), which no work type served
before.

## Files Changed

- Domain enums/defs: `mood`, `trait` (+`Generated`, `Rarity`, `Note`, `Beauty`),
  `stat`, `mentalbreak`, `job`, `command`, `work`, `skill.ForWork`, `item`
  (+`IsDrug`, `IsImplant`, values), `recipe`, `construction` (+`ForPrisoners`),
  `plant`, `research`, `incident`, `animal` (State fields, `Confused`), `health`
  causes, `pawngen` (`Rules.Legacy`, rarity weights).
- New packages (stubs): `pawn/social`, `pawn/body`, `pawn/drug`,
  `pawn/prisoner`, `animal/training`, `world/area`, `world/caravan`.
- Game: `phases.go`, `game.go`, `save.go`, `statehash.go`, `tick.go`,
  `priorities.go`, `combat.go`, `rooms.go`, `medical.go`, `handling.go`,
  `animals.go`, `raids.go`, `setup.go`, `incidents.go`, `zones.go`; new
  `rng_streams.go`, `social_state.go`, `people.go`, `social.go`,
  `social_grief.go`, `social_fight.go`, `social_mood.go`, `gathering.go`,
  `beds.go`, `body.go`, `surgery.go`, `drugs.go`, `prisoners.go`,
  `world_state.go`, `areas.go`, `bonds.go`, `history.go`, `views_social.go`.
- `statehash`: sparse encoding in `OfFields`.
- Ebiten: `renderer.go`, `inspector.go`, `app.go`, `colony_ui.go`,
  `archtools.go`, `architect.go`, `building_inspector.go`, `draw_art.go`,
  `work.go`; new `social_tabs_ui.go`, `social_art.go`.
- Docs: UI spec (ENG-003 subsections of §14, §15, §18, §27, §30, §33-35), art
  doc §37, `AGENTS.md` exception line, two checklist notes.

## Public Contracts Changed

All of C1-C4. Deviations: `PairKind` constants are `PairNone..PairExSpouse`
(the bare names clash with relation `Kind`s); the world container field is
`worldSt` (`g.world()` is the read model); the six read-model methods take
`entity.EntityID` like `Bio`; `beds.go` adds `sleepInPartnerBed`; `inArea` has
no call site yet (C3 names none).

## Invariants / Decisions

- Streams: `randx.New(seed ^ salt)`, lazy, saved in `RNGStreams` with the base
  seed under `"seed"` (hash ignores the seed entry).
- Priorities save as `PrioList` slices; 12-type `Prios` and 11-type `Prio`
  saves migrate (Warden 0).
- New crops are defined but not in `plant.Crops()` until their art exists; the
  four new build tools live in `archToolsPendingArt` with vector stand-ins.
- New incidents are never eligible (`incidentFits` false) until M5/M8.

## Verification
Tests: `go test -race -count=1 ./...`; determinism `-count=3`; goldens without
race (`TestScaleGolden` unchanged after the encoding re-pin).
Benchmarks: `BenchmarkTickGrown/120` about 82 us/op vs 79 us/op on f957fc8a
(+3-4 %, noisy; 250 and 500 within noise). Most of it is the larger saved
records (animal.State, mood.Memory) copied by value; the Doctor bill path costs
about 0.5 % behind `anyBillFor`, an order-independent existence check over the
bills map. This is the M0 baseline for later milestones.

## Determinism Impact

Goldens re-pinned twice, both explained in the tests: StateHash sparse encoding
(encoding only, computed on f957fc8a); `TestGrownGolden` again because
`CompleteResearch` finishes three new projects (field-by-field: only Research
moved). Legacy pawn generation is pinned (`TestLegacyRulesUnchanged`).

## Save / Mod Impact

SaveVersion stays 2; additive fields only; old saves load.

## Art / Visual Impact

No art added. Missing keys listed in ART_DIRECTION §37.

## Screen / UI Impact

Health/Social tabs show "No data yet."; Work tab gains a Warden column; Misc
architect category with marriage/party spots.

## Known Limitations

Every stub is a no-op. `staticcheck` in this environment is built with Go 1.23
and cannot load the module.

## Follow-ups

Wave A (M1, M2, M3-domain) fills the domain packages.

Progress: Wave A landed as SIM-039-body-surgery.md, SIM-040-drugs.md, SIM-041-social-core.md, UI-022-social-tab-history.md and ART-019-social-host-assets.md.
