# PRD-3: benches, work speed, art and sculptures

Date: 2026-10-29. Wave D. Backlog `.cursor/PRODUCTION_GAP_ANALYSIS.md` (sections 3.1, 3.3, 3.4). Contract commit `8f3932359` (arch(contract): wave D kinds, commands, jobs and read-model fields) landed the construction kinds, `BuildingView` Linked/WorkSpeed and the display names ("wood-fired forge", "electric forge", "powered door") before this work started.

## What changed
- **Recipes** (P3; `domain/economy/recipe/{recipe.go,benches.go}`): `Def.Also`, `Def.Builds`, `Input.Smelt`, `Base`, `At`, `SmeltYield`; 24 recipes appended. Workbench recipes stay at the workbench and run at wiki benches through `Also` (`RecipeView.Benches`, `RunsAt`). New: smithy blades and pila, crafting-spot bows, 13 machining-table guns, smelting metal weapons back to steel, three sculptures. The butcher spot gets the butchering bill.
- **Craft package** (`game/craft/`): `facility.go` and `station.go` (replace `cabinet.go`), `art.go`, `artgen.go`, `action.go`, `stockops.go`; a Workshop role in `deps.go`; `research.go`, `power.go` (+ spec test); `craft_host.go`; small edits in `fuel.go`, `save.go` (`Game.Art`), `building_view.go`, `action_view.go`.
- **Fuel:** the wood-fired forge holds 5 logs, burns 1 per 375 work only while worked, refuels below half up to full.
- **Power:** wiki watts for the five powered benches, Autocannon, UraniumSlugTurret and Autodoor; a placed and connected bench reads consumer and powered.
- **Work speed:** bench factor, facilities (tool cabinet +6 %), light, room (roof and cleanliness). Fills `BuildingView.Linked` and `WorkSpeed`.
- **Sculptures:** small 1x1, large 1x1 (11 stuff), grand 2x2 (10500 work). Quality and inspiration via PRD-2. Original generated title (`BuildingView.Epitaph`) and description. No minified-item path exists, so the carver sets the piece on the nearest free cell and the player moves it with the Move order.
- **Research:** 16 new projects with original names; a key under two projects needs both.
- Lead fix: inspector `RecipesFor` uses `RecipeView.RunsAt`; dead `RecipeBenchName` removed.

## Wiki numbers versus original tuning
- Wiki: bench footprints, costs and watts; bench factors (crafting spot 0.5, butcher spot 0.7); tool cabinet +6 %; dark work speed 0.8; outdoors 0.8; sculpture beauty 50/100/400; sculpture footprints and work.
- Original: the light ramp between 0 and 30 %; cleanliness factors 0.85/0.9/0.95; the art grammar and all names; research project names and grouping.

## UI
Representation by U4 (see also the wave D UI checklist lines): the bench pane shows Linked facilities and the WorkSpeed line; the recipe list follows `RunsAt`; 24 new Architect ToolDefs (production, furniture, security, powered door under Structure) with BuildSprite cases. Rendering: forge glow while burning, powered glow through PowerRole, stuff tint masks on sculptures.

## Art and sound
- A3: family `workshop` (benches, forges, spots, machining, art bench, smelter) and `fortify`; 161 sprites plus a re-render of the large and grand sculptures at the wiki footprints; icons for every kind; validator 5709 sprites, 0 errors; `make art-check` clean including orphans.
- S3: six bench work loops (smithy shared by both forges) in `cmd/audiogen/structure_recipes.go`; cues in `render/cue/structure_sounds.go`; audio 22.2 MB of 30 MB.

## Hysteresis
Forge refuel starts below half and fills to full; fuel burns only while worked (no-flap test in the craft tests).

## Tests and numbers
`craft/prd3_test.go`, `game/benches_prd3_test.go`, power spec test, `cmd/audiogen/structure_recipes_test.go`; journey `TestJourneySmithyBillWithFuel` (`journey_wave_d_test.go`). Coverage: craft 97.5 %, recipe 97.9 %. Game package 2 lines smaller.

## Gaps and follow-ups
- No minified items; `ArtDesc` is not in `BuildingView`, so the description shows only in the completion message, not in the inspector.
- Chemfuel (refinery) and wort/beer (fermenting barrel): items are missing, PRD-4. Also missing: fabrication bench, drug lab.
- Temperature penalty bands for stove and butcher spot (magnitudes not published).
- Report vs code: the report called the title field "generated title"; in code it is `BuildingView.Epitaph`.
