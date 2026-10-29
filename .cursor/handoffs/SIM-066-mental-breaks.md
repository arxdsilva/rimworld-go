# SIM-066: mental breaks that target others; the W3 integration

Date: 2026-10-29. Programme SIM-061…066 (`.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`). This handoff also records the integration, gates and budget for SIM-063…066. The four phases are one commit: the lead's contract files carry all four, so `git add` by path could not split them.

## What changed (M1, opus)
- **New sub-package `internal/application/game/mind`.** It owns break risk, choice, acting out and recovery, plus social fights (moved by P1). The game keeps one field, and `mental.go` is gone. Its six roles (Pawns, Breaks, Acts, Aggression, Brawls, Events) are adapted in `game/mind_host.go`.
- **Risk:** checked once an hour. Tiers sit at mood 35/20/5, shifted by the break-threshold traits; MTBs are 4/0.8/0.5 days. `mentalbreak.Pick` and `DurationFor` use the wiki tables (B1, SIM-061), with rolls on the social stream. A day of catharsis follows each break.
- **New breaks:**
  - **Insulting spree and targeted insulting spree:** insults go through the social interaction path and can start a social fight. The spree retargets at most every 250 ticks.
  - **Berserk:** attacks the nearest creature until downed or timed out.
  - **Murderous rage:** hunts one colonist or prisoner until it dies, or the rager is downed or arrested.
  - **Catatonic breakdown:** downs the pawn (`health.CauseCatatonic`, new), which is then rescued and fed like any patient.
- **Order menu:**
  - Any colonist may "Arrest <name>" a breaker; arresting ends the break (`g.mind.End` in `makePrisoner`).
  - A drafted colonist gets "Melee attack/Shoot <name>" on a berserk or murderous colonist, named plainly with no "raider". Such a row also opens the drafted menu (`orders.hasPawnOrder`).
- **UI:** the inspector's mental-break line names the target (`PawnView.MentalTarget`). The colonist bar tooltip names the break. Letters for aggressive breaks reach the stack.

## Integration (lead)
- **Components:** the care root was over 40 methods because the W3 contract stubs sat on it. Each feature now has its own component (`Feed`, `PrisonBeds`, `Guest`, `Visits`, `Strip`), and the root is back under 40. SOLID_GO §10 now says so, and allows adapters beside the wrapped code when the file budget is full.
- **Read-model fields:** `CarryingPawn`, `OnMedicalBed`, `GuestFaction`, `MentalTarget`, and `LetterView.JoinOffer`.
- **New command:** `AcceptLetter`. The UI's Accept/Decline buttons and a journey cover it, so `TestEveryInputActionHasAJourney` holds without a TODO entry.
- **P1 (opus), budget:** the new `internal/application/game/prison` sub-package took prisoner state, a prisoner's life and warden work. Brawls moved into `mind`. Recruiting and a guest joining share `g.enlist`. Three stone-view fields merged into one struct. No behaviour change: the hashes matched before and after.

## Numbers
| game package | SIM-062 commit | after W3 |
|---|---|---|
| non-test lines | 55,891 | 55,844 |
| files | 183 | 182 |
| `Game` methods / fields | 2,038 / 149 | 2,010 / 148 |

New sub-packages, with coverage: `care` ≈ 97 %, `mind` 96.4 %, `prison` 93.9 %.

Benchmark state hashes for the new behaviour: grown 250 `0xf6c61f40b377d985`, grown 500 `0x1a01790f2f095343`, raid 250 `0x8066e6b6f3956edd`, raid 500 `0x7185b71c1b1e1272`. These change intentionally from SIM-062: downed hunger, feeding, visits, rescue causes and breaks. Tick p99 is at or below the SIM-061 baseline (`SIM-061-bench-before.txt`).

## Gates
`go build`, `gofmt`, `go vet ./...`, `go test ./internal/arch/` (budgets lowered), full `go test -race ./...` with every journey, determinism and golden tests. `make lint` was not run: the installed staticcheck is too old for Go 1.26 (SIM-061). `TestGrownGolden` (internal/application/scenario) was re-pinned to `0x73d615f22d9b245`, its reason written beside the pin as the file's convention asks: the old pin held through SIM-062, and the new one is stable over `-count=2`.

## Open
- Tantrum/object destruction and escape breaks.
- Feeding downed animals.
- An event that brings faction-less strangers.
- A visit cooldown with saved per-pair state (today a fixed time window).
- "Resting (injured)" job line for guests.
- A drafted group right-click checks the first drafted pawn's options (SIM-062).

## Checklists
Vanilla 10.1 Mental break eligibility `[x]`, Destructive/aggressive breaks `[-]` (no tantrum/escape), Catatonic `[x]`.
