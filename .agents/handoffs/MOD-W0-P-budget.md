# MOD-W0-P: room in package `game`

Wave 0 of `MOD-000-mods-programme.md`. Moves code out of `internal/application/game` with no behaviour change, so the three mods have lines to spend.

## What moved

| New package | Component or content | Came from | Stays in `game` |
|---|---|---|---|
| `game/sense` | `Cleaning` (`Assign`, `Do`, `Wire`), `Wander` (`Pick`); roles `Filth`, `Ground`, `Walks`, `Labor`, `Strolls` | `clean.go`, `idleWander` | `sense_host.go`: adapters, `cleaningOf`, `wanderOf` |
| `game/dose` | `Drugs` (`State`, `Policy`, `SetPolicy`, `Tick`, `Assign`, `Do`, `Ingest`, `Overdose`, `Prophylaxed`, `Lines`, `Thoughts`), `Line`; roles `Pawns`, `Minds`, `Bodies`, `Stock` | `drugs.go` | `dose_host.go`: `savedDrug`, `exportDrugs`, `importDrugs`, `DrugPolicy`, adapters, `doseOf`. The field `Game.drugs` |
| `game/roster` | `CandidatePool`, `BioOf`, the bio and team read models, the Health tab's pure text (`PartStatus`, `CapacityTip`, `ConditionTip`) | `candidates.go`, `bio_build.go`, parts of `ng1_types.go`, `body.go`, `views_social.go` | Aliases, in `ng1_types.go` and `body.go` |
| `game/mapfields` | `Generate`, `Roofs`, `Fog`, `Regions`, `Layer` | `mapfields.go` | `MapFieldLayers`, the cache, aliases |

`savedDrug` stays in `game` because `testdata/save_schema.txt` names it. Components are values built where they are needed (as `snowOf`). Their adapters are one pointer each, so building one does not allocate, and there is no new `Game` field or method. `dose.New` takes `idleRecheck` as a parameter.

Per-tile path: `Cleaning.Assign` reads the Home cells as a slice. Its only interface call in the loop, `CleanFree`, comes after the distance filter, as the old reservation, postponed and area checks did. `DrugRolls` hands back the stream's draw at the moment the old code took it, so a lazily created stream appears at the same point.

Test call sites were repointed (`g.ingestDose` to `doseOf(g).Ingest`, and so on); no assertion changed. `candidates_test.go` moved to `roster`.

## Not moved

`wildlife_habitat.go`: its forage scan is a per-tile loop. `chunk_view.go` and `minimap_view.go`: they read `*WorldView`, a root type. `power*.go` has low coupling and about 1,300 lines, and is the next big payer, but its saved types and hot paths need a change of their own.

## For the docs

The gap documents' references to `game/drugs.go:NNN`, `game/clean.go` and `game/idle.go:109` now mean `game/dose/drugs.go`, `game/sense/clean.go` and `game/sense/wander.go`. CSE-010 and CSE-011 land in `dose.Drugs.Assign`. `assignDrugUse`, `nearestDrug`, `ingestDose` are `Assign`, `nearest`, `Ingest`.

## UI

None: nothing observable changed.

## Numbers and proof

The final report to the lead has the measured lines, methods, hashes and test results.
