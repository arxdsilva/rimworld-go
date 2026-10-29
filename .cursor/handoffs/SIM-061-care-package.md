# SIM-061: the care package, the sub-package rule, the pawn-interaction gap analysis

Date: 2026-10-29. Programme: colonist-to-colonist interaction at RimWorld fidelity (SIM-061…066). Backlog: `.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`. Plan: the user-approved plan, §3–§5.

## Why
The user asked for pawn-to-pawn interaction "as close as possible with RimWorld": select a colonist, right-click a downed one, Rescue to a safe bed. Most behaviours already existed as autonomous work; the gaps are player orders on a target pawn, rescue and bed fidelity, feeding patients, visiting the sick and mental breaks that target others. `internal/application/game` sat exactly at its SOLID budget, so the user decided to accept concise new sub-packages and move what belongs there first. This phase changes no behaviour.

## What changed
- **Rule** (`.cursor/SOLID_GO.md` §7, §8, §10; root `AGENTS.md`): a cluster may become a sub-package of the god type's package behind consumer-owned roles in its `deps.go`, with small adapters in `<sub>_host.go`; new features of a package at its ceiling go there. Guard edge in `internal/arch/solid_test.go`: a `game/<sub>` package never imports `game`. ENG-003 extended to pawn care and mental breaks (wiki numbers, original names and prose). `HEXAGONAL_ARCHITECTURE.md` §4 tree lists `game/care` and `game/mind`.
- **`internal/application/game/care`** (agent C1, opus): `Care` root, `Carries` (two-way carry index replacing `g.carrying` and its linear scan), `Policy`, `Doctor` (rescue, tend, emergency tend, `tendedUntil`), `Rest` (bed rest), `SafeBedFor` (one function reproducing the old rescue and bed-rest rankings), `PawnOrders` (today's "Tend X"), `CareView`. `Host` embeds eight value-only roles (`care/deps.go`); `game/care_host.go` implements them with eight adapter types and adds no `Game` method. Deleted from game: `care.go`, `bedrest.go`, `med_cmds.go`.
- **Domain contracts, inert** (lead): `command.Order` values `OrderRescue … OrderCarryToPrisonBed` (target in `Command.Target`); job kinds `FeedPatient`, `Strip`, `VisitSick`, `CarryToPrisonBed`; `PawnView.CarryingPawn` (filled in SIM-062).
- **Mental break tables** (agent B1, sonnet): new kinds `InsultingSpree`, `TargetedInsult`, `Berserk`, `MurderousRage`, `Catatonic` (appended); `Tier`, `Thresholds`, `ShiftThresholds`, trait offsets, `MTBTicks`, `Pick` with lower-tier fallback, `DurationFor`, `TargetsOthers`, `Downs`, all from the wiki. The existing `Kinds()`, `Threshold`, `RiskPerHour` and durations are unchanged, so the simulation does not use the new API until SIM-066.
- **UI prep** (agent U1, sonnet): `orders.hasPawnOrder` opens the menu for any pawn-targeted option (`EmergencyTend`, or `Prioritize` with a `Target`); the inspector shows "Carried by X" now and "Carrying X" once SIM-062 fills `CarryingPawn`.
- **Docs**: `.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md` (agent D1, sonnet; constants table filled by the lead from agent W1's wiki research), `DOCUMENT_INDEX.md` row, `MEDICAL_GAP_ANALYSIS.md` note on the moved code, vanilla checklist 9.1 Beds note corrected (medical beds exist; bed comfort is what's missing).

## Numbers
| | before | after |
|---|---|---|
| game non-test lines | 57,019 | 56,091 |
| game files | 185 | 183 |
| `game.Game` methods / fields | 2,105 / 152 | 2,047 / 149 |
| care package | — | ~1,000 lines, coverage 96.1 % |

Budget lines lowered to the measured values. From now on each phase must leave package game no larger than the previous commit.

Behaviour proof: `bin/benchmark -sizes 250,500 -speeds 3 -fixtures grown,raid -hash` on fixtures cached before the change (so loading them also proves save compatibility) gives the baseline hashes exactly: grown 250 `0x7d91c51db00ec208`, grown 500 `0xb525d0318b98b2d4`, raid 250 `0x584b5b06c1e2ffdc`, raid 500 `0xefdc3010260906b7`. Tick p99 at or below the baseline (`SIM-061-bench-before.txt`). `go build -gcflags=-m=2`: no caller local escapes through a host call.

## Gates
`go build ./...`, `gofmt -l` empty, `go vet ./...` clean, `go test ./internal/arch/` green with lowered budgets, full `go test -race ./...` (see the commit). **Not run:** `make lint`. The installed `staticcheck` was built with Go 1.23 and cannot load this Go 1.26 module, and fetching a newer one was not permitted in this session. Reinstall it with `go install honnef.co/go/tools/cmd/staticcheck@latest` and run `make lint`.

## Findings
- C1 accepted deviations: game calls the components directly (`g.care.Doctor.DoTend`), no root forwarders. `pickUpMedicine`/`tendMedicine` stay in game because vet work uses them. `SafeBedFor` takes a `BedNeed` (rescue or rest) because the two old rankings measure distance differently. `healingInjury` became a plain function.
- The wiki publishes no feeding trigger above Hungry, no downed food-fall rate, no guest-death goodwill and no visit-sick opinion minimum; the gap doc records each decision.
- The existing `needs.CriticalHunger` (2,500 of 10,000) already equals the wiki's Hungry threshold (0.25).

## Next
SIM-062: pawn-targeted orders (Rescue, Capture, Arrest, Prioritize tending) through `Prioritize` + `Target`, capture and arrest moving into care, journeys.
