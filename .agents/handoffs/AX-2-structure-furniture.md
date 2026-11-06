# AX-2: structure, furniture and decor

AX wave 1, 2026-11-02, agent A2. Every A2 kind is live: `construction.plannedA2` is now empty. ENG-003 applies, so the numbers are the wiki's (`scratchpad/wiki-ax.md`) and the names are original.

## Behaviour
- **Ornate door.** A 2-wide door (`IsDoor`). It faces along its wall by itself. Set over a wall run, it marks a wall on each cell for deconstruction and waits for both to come down (`placeFrameStuff`, `replacedStanding`). Its 50 gold (`Def.Extra`) is not delivered yet.
- **Column.** It blocks movement and holds a roof up to `roof.SupportRange` away, both in the full support field and in the incremental refresh (`roof.go`, `rooms_index.go`). It bounds no room.
- **Animal flap.** It is a door (`IsDoor`) that people can't pass:
  - Region flag `region.Flap` blocks the Colonist, Hostile, Sapper and Wild layers; Tame passes.
  - The fence index marks flaps `fenceFlap`, so tame animals plan on the Tame layer.
  - People plan with `flapQuery` (`colonistGrid`, used by `pathQuery`).
  - Raiders and wild animals see flaps among the shut doors.
  - It is never held open.
  - Penned animals' pen floods treat it as a boundary.
- **Double sleeping spot.** Two sleepers. It sleeps like the ground (`rest/waking.go`).
- **Animal sleeping box.** `AnimalSleeps`. Comfort 0.6, rest 0.93 (contract furniture table).
- **Tables.**
  - `Table` is the wiki 1x2: rotatable, wood, stone or steel, 75 HP, beauty 0.5.
  - The 2x2, 2x4 and 3x3 tables complete the set. `IsTable` is used for dining, social relax, room counts, cover and quality.
  - `construction.Seats` and `SeatCells` give one seat per side cell: 6, 8, 12 and 12. `rest.Seats.AtTable` seats diners there.
  - Save v11: `migrateBenches(resize.V11, RefundV10)` keeps an old 1x1 table that fits as 1x2, else takes it down and refunds it.
- **Couch.** Two seats: `IsSeat`, and `best` walks its footprint. Comfort 0.85.
- **Plant pot.**
  - `SetPotPlant` (Target, Build = `construction.PotPlant`: none, rose, daylily) picks or empties its plant. A colonist with Growing work sows it (`assignSow` takes `furnish.Pots.ToSow` first, `job.Sow` with the pot as Target; wiki sow work 750 and 540); another choice is replanted. Frost below -10 C kills it and it is sown again only at 0 C or warmer (no-flap test).
  - `game/furnish` grows the plant hourly in growing hours, by light and air (`furnishHost.GrowRate`). Rose takes 5 grow days, daylily 3.
  - Beauty (rose 14, daylily 18) scales with growth: `PotBeauty` feeds room tally, perceived beauty and the view.
  - State lives in `Building.Plant` and `Building.Growth`, saved with the building.
- **Small shelf.** A 1x1 `KindShelf` with 3 stacks.
- **Egg box.** A shelf zone for eggs only. A tame layer lays into the nearest one with room within 20 tiles that it can reach (`eggBoxNear`); otherwise the old pen rule applies.
- **Wall torch and wall lamp.** `WallHung`: each needs a built or planned wall (or rock) behind it, and auto-faces away from it.
  - Light radii: wall torch 6, wall lamp 8, flood light 17 (wiki 5.76, 7.54, 17.45).
  - The wall torch burns 15 wood at 2 a day and heats like the torch lamp.
  - The wall lamp and flood light draw power per `power/ax.go`.
  - Room lamp counts include all three.
- **Urn.** Unbuildable. Half the generated shed ruins hold one, from its own `ruin_urns` stream on a floored cell without loot, as a Ruin piece (A1's Claim takes it). Its card is locked with "Found in ruins, never built".
- **Steles.** Stone only, beauty 15 and 30. The stone mask tints them.
- **Walls.** Walls also take silver, gold, plasteel, uranium and jade:
  - stuff rows in `item/stuff.go`, `IsPreciousWall`;
  - hp and work % in `preciousStats`;
  - 10x cost and refund for silver, gold and jade (`CostFor` in the frame's Need and the Architect cost line, `RefundFor` in `refundOf`).

## UI
- **Architect.**
  - Cards for every kind.
  - The table card is a family (`ToolDef.Family`, `NextInFamily`). It stands for 1x2, 2x2, 2x4 and 3x3, and clicking it again arms the next size. A7 can reuse this for the bookcases.
  - Placement hints cover the hung lights and the new doors.
- **Inspector** (`inspector/ax2.go`):
  - seats; the pot's plant, growth and beauty, with Sow rose and Sow daylily buttons (`cmd.pot.<plant>`);
  - the hung light, column, flap and egg box lines;
  - Storage on the small shelf and egg box;
  - Hold open on the ornate door.
- **Render** (AR1's keys): `build_keys.go`, stone masks, cloth tints for the couch and flap, open flaps and doors, the wall torch flame, precious wall prefixes, and the pot plant drawn with the fire lily and dandelion sprites. `furniture/table_small` and `ui_icon/table` are retired.

## Budget
- Package game: 0 new Game methods (`colonistGrid`, `hangsOn` and `eggBoxNear` are functions).
- Lines are about -24 net. Moves paid for them:
  - eventlog's line text and kinds to `game/evlog`;
  - `workTypeOf` to `work.FromCommand`.

## Tests
- construction/ax_a2_test.go
- furnish/furnish_test.go
- game/ax2_test.go: flap with regions on and off, column, ornate door, hung lights, egg box, views, migration
- rest seats (rewritten for the 1x2 table) and the couch
- resize V11
- journey_ax2_test.go: five journeys
- The contract tests now allow live AX kinds.
- The determinism gate passes unchanged.

## Open (for the lead)
- **Plant pot.** Rose and daylily have no art of their own (AR).
- **Hung lights** take the object slot of the cell in front of the wall. A light left without its wall stays up.
- **Trader stock for urns** needs A1's crated buildings.
