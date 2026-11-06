# DEF-0: the combat package and the combat domain

Date: 2026-10-29. Programme BED/PRD/DEF (`.cursor/DEFENCE_GAP_ANALYSIS.md`). Wave A, committed with PRD-0 and BED-0.

## What changed
- **`internal/application/game/combat`** (X3, opus): components `Sight` (line of sight, cover), `Strikes`, `Guard` (defending), `Traps`, `Breach` and `Turrets`.
  - Host roles: Creatures, Rosters, Ground, Skills, Jobs, Harm, Events, Dice. Extra roles: Power, Flight, Hunts, Breaches. All are adapted in `game/combat_host.go`.
  - `turret.go` and `combat_state.go` are gone. The `turrets` field moved in and the save keeps its `Turrets` gob names.
  - Health, equipment, hunting work, building damage and repair, and path maps stay in game.
- **`internal/domain/combat/{damage,armor,attack,cover}`** (D3): damage types and their armor category, and armor resolution per the wiki.
  - Armor: effective rating minus penetration; below half it deflects, below the full value it halves, otherwise it passes. A halved sharp hit becomes blunt, layers resolve outer to inner, and the rating caps at 200 %.
  - Accuracy: shooter per-tile accuracy raised to the distance, times weapon accuracy bands (touch, short, medium, long), weather, smoke and cover.
  - Cover: sandbags and barricade 55 %, chunk 50 %, tree 25 %, wall and rock 75 %.
  - `weapon.Def` gained warmup, burst, penetration, damage type and bands. The 12 existing weapons reproduce today's formula exactly (`TestLegacyWeaponsReproduceToday`) until DEF-1 switches over.
- No behaviour change: the hashes are identical. Raid 500 tick p99 is within noise (0.66 vs 0.62–0.68 ms back to back), no caller local escapes, combat coverage 98.2 %.

## Wave A numbers (all three extractions)
| game package | before | after |
|---|---|---|
| lines | 55,844 | 54,621 |
| files | 182 | 181 |
| `Game` methods / fields | 2,010 / 148 | 1,950 / 147 |

Budget lines lowered. The three gap docs and their wiki constants are filled (G1, G2), from the W1–W3 research files. Checklist corrections made by the lead: vanilla 7.4 room stats and roles `[-]`, 16.3 ranged weapons `[-]`, warmup `[-]`, cooldown/flee/arrival/assault/kidnap/retreat `[x]`, storage priorities `[x]`, cooler note; UI missing-ingredient reason `[x]`. ENG-003 is extended to BED/PRD/DEF in `AGENTS.md`.

## Gates
`go build`, `gofmt`, `go vet ./...`, `go test ./internal/arch/`, full `go test -race ./...`, hashes. `make lint` is still pending a staticcheck built for Go 1.26.
