# AX-4: power and industry made real

Scope: the AX programme's A4 rows (`.cursor/ARCHITECT_GAP_ANALYSIS.md`), ENG-003 wiki numbers, original names. The lead's decision also made A4 the owner of `Def.Extra` delivery.

## Behaviour
- **Secondary materials (every kind).**
  - Each frame takes its `Def.Extra` materials in their own delivery slots, after the material and the components.
  - Haul keys gain `haul.ExtraKey(fid, i)` (bits 59-60). `haul.Slots`, `SlotOf`, `SlotFor`, `Give` and `ExtraView` replace the two-slot code in `haul_route.go`, and hand-ordered supply goes through them too.
  - `Frame.Ready` now also requires `ExtrasIn`. `ExtraNeed` and `ExtrasIn` are in `construction/ax_a4.go`; a reinstall or swap needs no extras.
  - `FrameView.AX.Extra`, `ExtraGot` and `Need` are filled. The status line reads "Needs N <item>", and the frame pane lists "Steel  0 / 2".
  - Cancelling a frame drops its extras. Deconstructing gives half back; floors are excluded because they use A1's refund rule.
- **Waterproof conduit.** It goes on shallow water (never deep) and joins conduits. It is drawn with AR2's 16-mask art.
- **Chemfuel generator.** 1000 W, a 30-chemfuel tank, 4.5 a day while running. Haulers refuel it with chemfuel only, and it reports "out of fuel" when empty.
- **Watermill.**
  - Map generation writes `mapgen.Result.River` (flow Dir8+1 on river water). It draws no RNG, and river-less maps are unchanged.
  - The game keeps that flow (saved).
  - The 5x6 mill's wheel row (`construction.WheelCell`) must sit in moving water.
  - Output: 1100 W, or 330 W when another mill is inside its 13x7 exclusion area.
  - The wheel layer turns while the mill makes power.
- **Deep ore.**
  - The ground-penetrating scanner is a researcher's job (`job.ScanGround`, under work.Research). A find comes every 1..360000 work units (hashed; mean 180000, wiki).
  - A find lays a lump with the wiki's sizes and yields; the weights and the ±25 % yield spread are original.
  - The deep drill is a miner's job (`job.OperateDrill`, under work.Mine). It runs 14000-work cycles and brings up the nearest ore within reach (21 cells). When none is in reach it brings up stone chunks, and a letter reports that the ore ran out. The portions per cycle are original tuning.
  - `WorldView.AX.DeepOre` and `DeepOreVer` are filled. The deep ore overlay (`overlay/deep_ore` times the ore colour) is forced on by a selected drill or scanner, or by their tools.
- **Moisture pump.** While powered it dries marsh and mud to soil and shallow water to gravel. Its radius grows one cell per 8.7 days, up to 6.9. Moving water and the sea are never dried.
- **Electric crematorium.** `Cremate` gains `Also: ElectricCrematorium`. It burns no wood, takes 180 work, starts with the default bill and shows Burning.
- **Panes.** Drill: "Drilling N%" and what it brings up. Scanner: "Scanning N%". Pump: "Dried radius N / 7 tiles". Watermill: "River flow N%".

## Code
- New `internal/application/game/industry`: river flow, the mill, deep ore, the operated machines, the pump, views and save. The wind band and the fuel table moved there from the game package to pay for the rest. The adapter `industryHost` lives in `power_gen.go`.
- The state lives in `bridgeState.ind`, beside the sea mask. `importOcean` now resets only the sea.
- The save field `AX4 industry.Save` is optional, so there is no version bump.
- Package game's net is about -13 lines and -3 Game methods (windClear, scanWindBand, windBlocked).

## Tests
- `industry/industry_test.go`, `haul/extra_test.go`, `mapgen/steps/water/flow_test.go`.
- `game/ax4_test.go`: extras delivery and refund, the waterproof conduit, the chemfuel burn and refuel, the watermill and its save, the drill, a no-flap test (losing power postpones the drill), the crematorium, the pump.
- `render/overlay/deep_ore_draw_test.go`.
- Journeys in `ebiten/journey_ax4_test.go`: every tool laid, panes, the overlay forced, and a river colony with the watermill and the waterproof conduit.
- Determinism gate: no golden moved. The mapgen goldens don't hash `River`.

## Not done (noted in the gap rows)
- No insect infestation from drilling.
- The chemfuel generator doesn't explode when damaged, and it emits no heat or smoke.
- The crematorium has no apparel, weapon or drug bills and no heat.
- Older saves and river-less maps have no flowing water.
