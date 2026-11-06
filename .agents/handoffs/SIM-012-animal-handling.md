# SIM-012 Animal handling: taming, pens, guards, manhunters

## What changed
- `domain/animal`: `TameChance`, `Revenge`, `TameRevenge` per species; `TameWork`,
  `ManhunterTicks` (a day), `TameRetryTicks`; State `Tame`, `Name`, `TameMark`,
  `TameRetry`, `Manhunter` (+ `Enraged(now)`).
- `internal/application/game/handling.go`:
  - `DesignateTame` (and cancel), `assignTame` under Hunt work, `job.Tame`/`doTame`.
    A reserved animal holds still. Success names it; failure sets a retry delay and
    may `enrage` it (danger message, so the game pauses).
  - `provoked`: wild animals hurt by a colonist may go manhunter. `thinkManhunter`
    hunts the nearest standing colonist anywhere until the rage ends.
  - Tame animals (`thinkTame`): wolves guard (raiders, hostile or enraged animals
    within 10); grazers flee only wild predators; all keep to the nearest pen, or
    within ~12 tiles of the colony's first bed. They path with the real grid (doors
    open for them), never spring traps, and are not "hostile".
  - Pens: `DesignatePen` rectangles (`g.pens`, saved, WorldView `Pens`), removed by
    Cancel. `ReleaseAnimal` command. Storyteller: outside winter, a quarter of threats
    from day 8 are manhunter dustbuck packs (`ManhunterPack` dev hook).
  - `chase` is the shared close-in-and-bite helper for predators, manhunters, guards.
- Adapter: Orders > Tame, Zones > Animal pen; tame/manhunter overlays (heart, red
  chevron); names under tame animals; animal inspector Tame / Release / Slaughter;
  Animals tab (`animals_ui.go`) with slaughter and release; pen wash + fence.
  New art: `overlay/designation_tame`, `overlay/pen_zone`, icons `order_tame`,
  `zone_pen`. `-scenario animals` with `-capture-ui pen|animals`.

## Tests
`handling_test.go` (taming, release, manhunter, provoked odds, guard wolf, pens
through a door, cancel), `animals_ui_test.go`.
