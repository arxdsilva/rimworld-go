# mind package

## Package Purpose
Mental breaks: the hourly break check (wiki tiers, MTB, trait-shifted thresholds), the minor breaks (sad wander, food binge, hide in room), the breaks aimed at others (insulting spree, targeted insults, berserk, murderous rage), the catatonic collapse, and the course of a social fight (its brawl job: closing in and the punches). A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by SIM-066; the gap backlog is `.cursor/PAWN_INTERACTION_GAP_ANALYSIS.md`.

## Owned Concepts
- `Mind` (mind.go): built by `New(Host)`; `Tick(id, j)` (the AI phase, standing colonists), `Downed(id)` (the AI phase's downed branch and every blow), `End(id)` (an arrest), `Force(id, k)` (dev tools), `Target(id)` (read model), `Thresholds(id)`, `Export`/`Import`, `StartFight`/`EndFight`/`InFight`/`Brawl` (fight.go; `Brawl` runs the brawl job in the game's job phase).
- `Track`: a break's target and its next-retarget / next-insult tick; `Save` also keeps each colonist's calm window (no new break during the catharsis day).
- `Hostile(k)`: berserk and murderous rage make a colonist a fair attack target (the game's `standingRaider`/`raiderAt`).
- Tantrum (CBT-4, tantrum.go): a major break (wiki: 8000-12000 ticks, commonality 0.333, the 20 % threshold and 0.8-day MTB of its tier; violent colonists only): it picks a building or item stack at random within `TantrumRadius` (8, original), walks beside it and punches it on its fist's cycle until it breaks (`Smash` role: `Breakable`, `GoBeside`, `Smash`; `Track.X/Y`). Read model: `PawnView.MentalBreak` "tantrum".

## Allowed Dependencies
- Domain value types (`pawn/mentalbreak`, `pawn/mood`, `pawn/social`, `ai/job`, `sim/entity`, `sim/clock`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.

## Forbidden Dependencies
- `internal/application/game`, adapters, platform, Ebitengine, global RNG, wall clock.

## Public Contracts
- `deps.go` (CBT-4 adds Smash (3)): `Host` embeds six roles, each ≤ 8 value-only methods: Pawns (8), Breaks (4), Acts (8), Aggression (6), Brawls (8), Events (4). The game implements them in `internal/application/game/mind_host.go`; none adds a method to `game.Game`.
- The break record (kind, until) stays on the game (`g.breaks`, saved as `Breaks`), since dementia writes it too; mind's own state is the save's `Mind` field.
- Numbers: thresholds 35/20/5, MTB 4/0.8/0.5 days, durations and weights come from `mentalbreak` (wiki, ENG-003 extended). `RetargetTicks`, `InsultGap`, `InsultRange`, `BerserkRadius` are original tuning.

## Invariants
- Every roll comes from `Host.Roll` (the game's social stream); no wall clock; no map iteration.
- Hysteresis: a break lasts its drawn duration; a spree or berserker retargets at most every `RetargetTicks`; insults are `InsultGap` apart; no new break during the calm window after one ends.

## Tests
- Unit tests here run against `fakeHost` (host_test.go); no-flap tests `TestSpreeDoesNotFlap`, `TestBerserkDoesNotFlap`, `TestCatharsisBlocksANewBreak`, `TestTantrumDoesNotFlap` (tantrum_test.go).
- Whole-game tests: `internal/application/game/mental_targets_test.go`, `mental_test.go`, `fight_test.go`.

## Hot Paths
- `Tick` runs per standing colonist per tick: off its hourly check tick a calm colonist costs one map lookup and no roll. `Downed` runs per downed pawn per tick: one map lookup. `Brawl` runs per fighter per tick: value host calls only.
