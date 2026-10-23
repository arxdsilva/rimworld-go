# SIM-034 Components, industrial medicine, tool cabinet, hospital bed and pod shells

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md` §4.8, §5.6; decision D13). Commits: `cf293081` (rules), `f815d545` (in play), `f7bebeb1` (art).

## What changed

### Items and recipes
- **Components** (`item.Components`): made at the workbench by the recipe *Fabricate component* (12 steel into 1, 1500 work, Crafting skill, minimum Crafting 6, behind the Fabrication research), or mined from **buried machinery** ore veins (`geology.Machinery`, only on site maps; one tile yields 2 components; ore sprite `rock/ore_machinery_*`).
- **Industrial medicine** (`item.IndustrialMedicine`): the recipe *Refine industrial medicine* (2 herbal medicine into 1, 800 work, a doctor's job at the workbench, minimum Medicine 4, behind the Pharmacology research).
- Every recipe now names the skill it trains (`Learn`) and an optional minimum skill (`MinSkill`); pawns below it do not take the bill. Stockpiles accept the new goods.
- **Research:** *Fabrication* (cost 5000, needs Gunsmithing; unlocks the recipe, tool cabinet, hospital bed) and *Pharmacology* (4000, needs Herbalism). Saves keep research as a growable list; v1 saves still load (`research.V1`).

### Buildings
- **Components on frames:** a building definition can declare a component cost. The frame needs its material first, then its components, delivered by haulers (`supplyKind`) before work can start; cancelling returns them, deconstruction returns half rounded down.
- **Tool cabinet** (`construction.ToolCabinet`): steel only, 30 steel, 1 component. Each cabinet within 6 tiles of a bench adds 6 % to its bill work speed, at most two count (cached per structure version).
- **Hospital bed** (`construction.HospitalBed`): 45 wood or steel, 2 components, 1x2, a bed that also counts as a sleeping spot. A patient lying in it gets 115 % of a tend's healing and half the wound infection chance; rescuers prefer a hospital bed.
- **Steel furniture:** beds, double beds, tables and chairs can now be built of steel as well as wood.
- **Pod shell** (`construction.PodShell`): placed by the arrival for each drop pod (walkable, never designated); deconstructing it salvages 6 steel.

### Medicine tiers (`pawn/infection`, `game/items_ng1.go`)
Care tiers are untreated, self-tended, tended without medicine, herbal medicine and industrial medicine. A doctor uses industrial medicine for an injury of severity 50 or more, or any infection, and herbal medicine otherwise, each falling back to the other when out of stock (`TestMedicineChoiceNoFlap`). Industrial medicine heals a tend 2.5x (herbal 2x, self-tend half), leaves a wound infected 1 % of the time (herbal 3 %), and takes 12 off infection severity per hour (herbal 9).

### UI and art
Architect cards for the hospital bed and tool cabinet show components in their tooltips; a scenario can forbid buildings (shown disabled with the reason). Art in `f7bebeb1`: component and industrial-medicine stacks (small, medium, large), tool cabinet, hospital bed in four facings, drop-pod shell and effects, machinery ore, and icons.

## Verification
- Recorded at commit time; the docs pass re-ran nothing. Domain tests: `recipe/advanced_test.go`, `construction/advanced_test.go`, `research/advanced_test.go`, `infection/industrial_test.go`. Game tests (`game/water_test.go`): `TestComponentFramesWait`, `TestCancelledFrameReturnsComponents`, `TestMachineryOreYieldsComponents`, `TestDeconstructRefundsHalfComponents`, `TestPodShellDeconstructsToSteel`, `TestMedicineChoiceNoFlap`, `TestIndustrialCareHeals`, `TestToolCabinetBonusCap`, `TestHospitalBedHeal`, `TestRescuePrefersHospitalBed`, `TestStockpilesTakeNewGoods`.

## Known limitations
- Silver is still not currency: there is no trade (out of NG-1 scope), so "silver becomes the currency" (D13) has no effect yet.
- No medical-care policy per patient: the doctor chooses the tier by severity.
- The tool cabinet bonus applies to workbench bill work; it does not speed other work.
- Components are only made at the workbench and mined; there is no machining bench.
- No power, so no powered medical or fabrication equipment.
