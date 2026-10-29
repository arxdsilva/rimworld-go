# SIM-062: orders on a pawn (rescue, capture, arrest, prioritize tending)

Date: 2026-10-29. Programme SIM-061…066; backlog `.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`; previous phase `SIM-061-care-package.md`.

## What the player gets
Select a colonist, right-click a pawn. The menu lists, per pawn on the tile:

- "Rescue <name>" for a downed colonist out of bed;
- "Capture <name>" for a downed raider or visitor;
- "Arrest <name>" for a standing colonist or visitor;
- "Prioritize tending <name>" when undrafted, "Tend <name>" when drafted.

Disabled rows say why: "no free bed", "no free prisoner bed", "carried by <name>", "cannot reach", "not able to act" or "cannot target itself". Shift queues any of them behind the current order, and the queue names the target. A forced rescue works drafted or undrafted, the rescuer stays drafted, and the patient goes to the bed `SafeBedFor` picks. That is today's ranking; SIM-063 changes it.

## How
- **Contract (lead).** Pawn-targeted orders are `Prioritize` with `command.Order.TargetsPawn()` and the target in `Command.Target`. `tick.go` and `execOrder` route them to `care.Force`. `EmergencyTend` joins the Shift-queue path; its own queue clear moved to `tick.go`'s plain-order path, so a queued tend no longer wipes the orders behind it. `orderLabel` also matches `Target`. Other contract pieces:
  - `care.Prison`, a second constructor role with 8 methods: raider/visitor checks, person name, prison bed, drop gear, make prisoner, arrest roll on the game's prison stream, start fight.
  - `Events.Message`, `Jobs.Breaking`, reason constants in `care/reasons.go`, `CareView.Carrying` → `PawnView.CarryingPawn`.
- **S1 (opus).** `care/orders.go` and `care/forced.go`: one `judge(doer, order, target)` decides both the menu row and `Force`, so they cannot disagree. 51 fixtures check each case twice. A forced rescue claims the patient, so the autonomous Doctor does not race it.
- **S2 (opus).** Capture and arrest moved into care (`care/capture.go`), and `prisoner_capture.go` is deleted. `carryToBed` (`care/carrybed.go`) is the one carry-to-bed driver for rescue and capture. `Arrestable` checks roster membership as the old code did, so a released prisoner walking out is not arrestable.
- **U2 (sonnet).** UI path verified: the undrafted and drafted right-clicks open the menu, disabled rows only hint, Shift queues any kind, and the inspector Capture/Arrest buttons still work. Tests only; no production change was needed.
- **J2 (sonnet).** Journeys `TestJourneyOrdersRescueDowned`, `…CaptureFromMenu`, `…ArrestFromMenu`, `…PrioritizeTend`, `…RescueQueued`. 13 whole-game `TestPawnOrders*` tests: no bed, carried, drafted and undrafted rescue, plain-bed fallback, the Doctor not stealing an ordered target, the patient picked up by someone else or dying, the capture and arrest rows, tend by draft state, queued rescue. Lead tests: `TestEmergencyTendQueues`, `TestOrderLabelMatchesTarget`.

## Numbers
- Game package: 56,091 → 55,891 lines, 183 files, `Game` methods 2,047 → 2,038, fields 149. Budget lines lowered.
- care package: coverage 97.5 %.
- Benchmark state hashes are unchanged: grown 250 `0x7d91c51db00ec208`, grown 500 `0xb525d0318b98b2d4`, raid 250 `0x584b5b06c1e2ffdc`, raid 500 `0xefdc3010260906b7`. The fixtures place no orders, so this proves the moves changed no autonomous behaviour.

## Gates
`go build`, `gofmt`, `go vet`, `go test ./internal/arch/`, full `go test -race ./...`. `make lint` was not run: the installed staticcheck is too old for Go 1.26, as noted in SIM-061.

## Decision found at the gate
The first full run failed `TestDraftedRightClickOffersTend`: every standing colonist offers "Arrest <name>", so a drafted right-click onto a colonist's tile opened the menu instead of walking there. `orders.hasPawnOrder` no longer counts Arrest when deciding to force the drafted menu. Arrest still appears whenever the menu opens, and always on an undrafted right-click (`TestHasPawnOrder` "arrest alone").

## Known gaps, left for later
- A drafted group right-click checks the first drafted pawn's options, which may differ from the primary pawn the menu acts for (U2).
- RimWorld tends only a lying patient; "Prioritize tending" here also offers a standing wounded colonist (S1). SIM-063 may align it.
- Rescue covers downed colonists only; guests, allies, strangers and every downed cause come in SIM-063.

## Checklists
- Vanilla 5.5 right-click forced work and queued forced jobs: notes extended.
- Vanilla 11.5 Rescue: `[x]` → `[-]`. Its acceptance names "friendly" pawns; guests arrive in SIM-063.
- UI 13.2 Rescue/capture contextual commands: `[x]`. UI 3.2 context-sensitive actions: note extended.
