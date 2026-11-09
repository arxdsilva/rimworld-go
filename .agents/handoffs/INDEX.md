# Handoff index

One row per handoff section. Open the ledger at `#<lowercased ID>`. A new task appends its section to the ledger of its family (below; start a new ledger only when none fits, never named `AGENTS.md`) and adds its row here: `python3 scripts/handoff_index.py` rebuilds this file. Format: `.agents/templates/HANDOFF.md`.

| Ledger | Families |
|---|---|
| [AI.md](AI.md) | AI |
| [ARCH.md](ARCH.md) | ARCH |
| [ART.md](ART.md) | ART, SHADOW, CURSOR, VIEW |
| [AX.md](AX.md) | AX |
| [BASE.md](BASE.md) | PRD, PWR, REF, TEMP, RSCH, BED, WRK, SURV, RES |
| [DEF.md](DEF.md) | DEF, CBT, SHT |
| [DOS.md](DOS.md) | DOS |
| [ENG.md](ENG.md) | ENG |
| [FOOD.md](FOOD.md) | FOOD, FEED, FISH |
| [GEAR.md](GEAR.md) | GEAR |
| [MED.md](MED.md) | MED, AIL |
| [MOD.md](MOD.md) | MOD, CSE |
| [NG.md](NG.md) | NG (the NG1 new-game plan) |
| [OPS.md](OPS.md) | CI, DEPS, REL, UPDATE, USAGE, LOG, TOKEN, DOCS, REVIEW, QA, SYS |
| [PERF.md](PERF.md) | OPT, PERF |
| [PLAY.md](PLAY.md) | PLAY (handoffs and the PLAY010–PLAY024 plans) |
| [SIM.md](SIM.md) | SIM |
| [SND.md](SND.md) | SND |
| [STORAGE.md](STORAGE.md) | STK, NST, STO, ZONE, HAUL |
| [UI.md](UI.md) | UI, CTL, TIME, UIS |
| [WORLD.md](WORLD.md) | MAP, WG |

| ID | Title | Date | Ledger |
|---|---|---|---|
| AI-001-agents-folder | One `.agents/` folder for every agent tool | 2026-11-06 | [AI.md](AI.md#ai-001-agents-folder) |
| AI-002-no-tool-folders | Agent files live only in `.agents/`; the repo keeps no tool folder | 2026-11-08 | [AI.md](AI.md#ai-002-no-tool-folders) |
| AIL-1-stuck-downed | No exposure state holds a colonist down forever | 2026-11-06 | [MED.md](MED.md#ail-1-stuck-downed) |
| AIL-2-open-air | Heatstroke victims are carried into cool open air | 2026-11-06 | [MED.md](MED.md#ail-2-open-air) |
| ARCH-001-solid-rule-and-guard | SOLID rule and the arch guard | 2026-10-28 | [ARCH.md](ARCH.md#arch-001-solid-rule-and-guard) |
| ARCH-002-in-package-preparation | In-package preparation for the ebiten decomposition | 2026-10-28 | [ARCH.md](ARCH.md#arch-002-in-package-preparation) |
| ARCH-003-shared-leaves | The shared leaves leave the ebiten root | 2026-10-28 | [ARCH.md](ARCH.md#arch-003-shared-leaves) |
| ARCH-004-bench-4.1 | Benchmark record after wave 4.1 | 2026-10-28 | [ARCH.md](ARCH.md#arch-004-bench-4.1) |
| ARCH-004-renderer-layers | The renderer's draw layers leave the root | 2026-10-28 | [ARCH.md](ARCH.md#arch-004-renderer-layers) |
| ARCH-005-app-features | The App's features leave the root | 2026-10-28 | [ARCH.md](ARCH.md#arch-005-app-features) |
| ARCH-006-sweep | The sweep: the root keeps only the composition | 2026-10-28 | [ARCH.md](ARCH.md#arch-006-sweep) |
| ARCH-007-cmd-wiring-only | Every `cmd/<tool>` is one wiring-only `main.go` (GAR Part A) | 2026-11-08 | [ARCH.md](ARCH.md#arch-007-cmd-wiring-only) |
| ARCH-008-gar-analysis-adrs | GAR gap analysis, ADRs 0001-0004 and the architecture diagrams (GAR Parts B and D) | 2026-11-08 | [ARCH.md](ARCH.md#arch-008-gar-analysis-adrs) |
| ARCH-009-phase-scheduler | The tick's phases are declared systems in a schedule (GAR-001..005, 012) | 2026-11-08 | [ARCH.md](ARCH.md#arch-009-phase-scheduler) |
| ARCH-010-path-stage-pool | Animal repaths as a decide-then-apply path stage on an 8-lane pool (GAR-035, GAR-031) | 2026-11-08 | [ARCH.md](ARCH.md#arch-010-path-stage-pool) |
| ART-000-agent-pipeline-integration | Art spec wired into the agent pipeline | 2026-09-08 | [ART.md](ART.md#art-000-agent-pipeline-integration) |
| ART-001-vendor-pack-renderer | CC0 vendor character pack in the renderer | 2026-09-08 | [ART.md](ART.md#art-001-vendor-pack-renderer) |
| ART-002-presentation-animation-clock | Presentation animation clock | 2026-09-08 | [ART.md](ART.md#art-002-presentation-animation-clock) |
| ART-003-pawn-interpolation | Pawn interpolation between ticks | 2026-09-08 | [ART.md](ART.md#art-003-pawn-interpolation) |
| ART-004-animation-pass | Animation pass: pawns, animals, combat, work, world | 2026-10-06 | [ART.md](ART.md#art-004-animation-pass) |
| ART-004-main-menu-colonist | Main menu colonist bust | 2026-09-15 | [ART.md](ART.md#art-004-main-menu-colonist) |
| ART-005-blender-pipeline-style-prototype | Blender pipeline and style prototype (ART Wave 0 frozen) | 2026-10-05 | [ART.md](ART.md#art-005-blender-pipeline-style-prototype) |
| ART-006-original-art-in-renderer | Generated art in the ebiten renderer | 2026-10-05 | [ART.md](ART.md#art-006-original-art-in-renderer) |
| ART-007-smooth-animation | Smooth movement and body animation (ART §27.1) | 2026-10-19 | [ART.md](ART.md#art-007-smooth-animation) |
| ART-008-action-animations | Action animations: holding, combat, hits, work (ART §27.2) | 2026-10-19 | [ART.md](ART.md#art-008-action-animations) |
| ART-009-living-world | Living world: wind, weather, vegetation, burial (ART §9, §16, §27.3, §28) | 2026-10-19 | [ART.md](ART.md#art-009-living-world) |
| ART-010-normals-light-shadows | Normal maps, sun/moon light, moving shadows | 2026-10-20 | [ART.md](ART.md#art-010-normals-light-shadows) |
| ART-011-animal-roster | Animal roster: 61 species from one parametric rig | 2026-10-21 | [ART.md](ART.md#art-011-animal-roster) |
| ART-011-menu-colony-diorama | Main menu dusk colony diorama | 2026-10-21 | [ART.md](ART.md#art-011-menu-colony-diorama) |
| ART-012-creature-shadows | Pawn and animal silhouette shadows | 2026-10-21 | [ART.md](ART.md#art-012-creature-shadows) |
| ART-013-biome-terrain-plants-rest-pose | Biome terrain, plants, items, rest pose | 2026-10-22 | [ART.md](ART.md#art-013-biome-terrain-plants-rest-pose) |
| ART-014-fences-forbid-icons | Fences, gates, forbidden overlay, icons | 2026-10-22 | [ART.md](ART.md#art-014-fences-forbid-icons) |
| ART-014-pawn-builds-portraits-water | Pawn builds, storyteller busts, water, NG-1 world art | 2026-10-22 | [ART.md](ART.md#art-014-pawn-builds-portraits-water) |
| ART-015-baked-corner-linked-rock-stone-tints | Linked walls and rock: baked corners, stone tints | 2026-10-22 | [ART.md](ART.md#art-015-baked-corner-linked-rock-stone-tints) |
| ART-016-vegetation-restyle | Vegetation restyle (PLAY-010) | 2026-10-23 | [ART.md](ART.md#art-016-vegetation-restyle) |
| ART-017-trees | Trees (PLAY-010) | 2026-10-23 | [ART.md](ART.md#art-017-trees) |
| ART-018-xs-animal-size | XS animal size (PLAY-010) | 2026-10-23 | [ART.md](ART.md#art-018-xs-animal-size) |
| ART-019-social-host-assets | Social and host-system art (ENG-003) | 2026-10-24 | [ART.md](ART.md#art-019-social-host-assets) |
| ART-020-vents-ruins | Steam vents and ruins art (PLAY-013, MAPGEN-2 §3.4-3.5) | 2026-10-24 | [ART.md](ART.md#art-020-vents-ruins) |
| ART-021-bridge-duskmoss-casket | Bridge, duskmoss, sleeper casket art (PLAY-014, MAPGEN-3) | 2026-10-24 | [ART.md](ART.md#art-021-bridge-duskmoss-casket) |
| ART-022-crops-restyle | Crops restyle (PLAY-016, ART §9.3) | 2026-10-25 | [ART.md](ART.md#art-022-crops-restyle) |
| ART-023-storyteller-portraits-raster | Storyteller portraits as painted busts | 2026-10-26 | [ART.md](ART.md#art-023-storyteller-portraits-raster) |
| ART-024-ui-chrome-charcoal | UI chrome to neutral charcoal with gold accent | 2026-10-26 | [ART.md](ART.md#art-024-ui-chrome-charcoal) |
| ART-025-rimworld-look | RimWorld-style look: camera, outlines, palette, limbless pawns | 2026-10-26 | [ART.md](ART.md#art-025-rimworld-look) |
| ART-026-medical-items | Medical item art, vitals monitor, care icons (MED-05/06) | 2026-10-26 | [ART.md](ART.md#art-026-medical-items) |
| ART-027-held-items-rimworld | Weapons and items held the RimWorld way | 2026-10-26 | [ART.md](ART.md#art-027-held-items-rimworld) |
| ART-028-menu-art-original | Main-menu hero replaced with original art (clean-room fix) | 2026-10-26 | [ART.md](ART.md#art-028-menu-art-original) |
| ART-029-menu-hero-extended | Menu hero extended, cut tool hardened | 2026-10-27 | [ART.md](ART.md#art-029-menu-hero-extended) |
| ART-030-play022-animals-puddles | Animal size, animation and puddles (PLAY-022) | 2026-10-27 | [ART.md](ART.md#art-030-play022-animals-puddles) |
| ART-MEAT-steaks | Meat reads as steaks | 2026-11-03 | [ART.md](ART.md#art-meat-steaks) |
| AX-0-contract | Inert contract for every vanilla Architect item | 2026-11-01 | [AX.md](AX.md#ax-0-contract) |
| AX-1-orders-and-zones | Orders and zones, Remove floor | 2026-11-02 | [AX.md](AX.md#ax-1-orders-and-zones) |
| AX-2-structure-furniture | Structure, furniture and decor | 2026-11-02 | [AX.md](AX.md#ax-2-structure-furniture) |
| AX-3-floors | Every vanilla floor, made real | 2026-11-02 | [AX.md](AX.md#ax-3-floors) |
| AX-4-power-industry | Power and industry made real | 2026-11-02 | [AX.md](AX.md#ax-4-power-industry) |
| AX-5-foam-security | Fire foam and security | 2026-11-02 | [AX.md](AX.md#ax-5-foam-security) |
| AX-6-recreation | Recreation | 2026-11-02 | [AX.md](AX.md#ax-6-recreation) |
| AX-7-books | Books and bookcases made real | 2026-11-02 | [AX.md](AX.md#ax-7-books) |
| AX-8-escape-ship | The escape ship, end to end | 2026-11-02 | [AX.md](AX.md#ax-8-escape-ship) |
| AX-9-pods-casket-scanner | Stasis casket, transport pods, mineral scanner, hitching spot | 2026-11-02 | [AX.md](AX.md#ax-9-pods-casket-scanner) |
| AX-AR1-art | A2/A3 art: decor, furniture, walls, floors | 2026-11-01 | [AX.md](AX.md#ax-ar1-art) |
| AX-AR2-art | Art for orders (A1), power/industry (A4), fire foam/security (A5) | 2026-11-01 | [AX.md](AX.md#ax-ar2-art) |
| AX-AR3-art | A6/A7 art: recreation, books, bookcases | 2026-11-02 | [AX.md](AX.md#ax-ar3-art) |
| AX-AR4-art | Art for the escape ship (A8) and caskets, pods, scanner, hitching spot (A9) | 2026-11-02 | [AX.md](AX.md#ax-ar4-art) |
| BED-0-rest-package | The rest package, stats and room formulas | 2026-10-29 | [BASE.md](BASE.md#bed-0-rest-package) |
| BED-1-bed-stats | Bed stats, bedrolls, animal beds, bed furniture, forbidding buildings | 2026-10-29 | [BASE.md](BASE.md#bed-1-bed-stats) |
| BED-2-room-stats-roles-thoughts | Room statistics, roles and room thoughts | 2026-10-29 | [BASE.md](BASE.md#bed-2-room-stats-roles-thoughts) |
| BED-3-beauty-comfort-outdoors-needs | Comfort, beauty and outdoors needs | 2026-10-29 | [BASE.md](BASE.md#bed-3-beauty-comfort-outdoors-needs) |
| CBT-1-damage-model | Damage model | 2026-10-30 | [DEF.md](DEF.md#cbt-1-damage-model) |
| CBT-2-attack-mechanics | Attack mechanics | 2026-10-30 | [DEF.md](DEF.md#cbt-2-attack-mechanics) |
| CBT-3-weapons-gear-structures | Weapons, gear and structures | 2026-10-30 | [DEF.md](DEF.md#cbt-3-weapons-gear-structures) |
| CBT-4-combat-ai | Combat AI | 2026-10-30 | [DEF.md](DEF.md#cbt-4-combat-ai) |
| CBT-5-explosions-and-combat-ui | Explosions and combat UI | 2026-10-30 | [DEF.md](DEF.md#cbt-5-explosions-and-combat-ui) |
| CI-001-race-budget-and-memory | The `-race` suite within CI's time and memory | 2026-11-02 | [OPS.md](OPS.md#ci-001-race-budget-and-memory) |
| CI-002-tint-shader-race | The tint shader's first draw raced | 2026-11-04 | [OPS.md](OPS.md#ci-002-tint-shader-race) |
| CI-003-game-test-time | `application/game` test time under `-race` | 2026-11-06 | [OPS.md](OPS.md#ci-003-game-test-time) |
| CI-004-ebiten-test-time | The ebiten root's `-race` test time | 2026-11-06 | [OPS.md](OPS.md#ci-004-ebiten-test-time) |
| CI-005-heavy-binaries-serial | CI headroom: heavy test binaries alone, catalog parsed once, journeys skip wasted world work | 2026-11-08 | [OPS.md](OPS.md#ci-005-heavy-binaries-serial) |
| CSE-W1-habits | Options Habits tab (CSE-049, CSE-023) | 2026-11-03 | [MOD.md](MOD.md#cse-w1-habits) |
| CSE-W2-pack | The pack (CSE-036) | 2026-11-03 | [MOD.md](MOD.md#cse-w2-pack) |
| CSE-W3-tidy | Tidying a room around a job (CSE-017…027) | 2026-11-04 | [MOD.md](MOD.md#cse-w3-tidy) |
| CSE-W4-bills | Bills gather in the pack (CSE-029…035, 040, bill call of CSE-018) | 2026-11-04 | [MOD.md](MOD.md#cse-w4-bills) |
| CSE-W5-gear | Spare meal, put back, manual unload (CSE-005, 037, 038, 039) | 2026-11-04 | [MOD.md](MOD.md#cse-w5-gear) |
| CSE-W6-food | Food (CSE-012…015) | 2026-11-04 | [MOD.md](MOD.md#cse-w6-food) |
| CSE-W7-joy | Joy and drugs (CSE-001, 002, 007, 008, 010, 011) | 2026-11-04 | [MOD.md](MOD.md#cse-w7-joy) |
| CSE-W8-wander | Wander, snow sky, cold store (CSE-028, 041, 042, 043) | 2026-11-04 | [MOD.md](MOD.md#cse-w8-wander) |
| CSE-W9-recipes | Recipe ingredient kinds named (CSE-046) | 2026-11-04 | [MOD.md](MOD.md#cse-w9-recipes) |
| CTL-001-mouse-controls | Mouse controls (CTL-001..020) | 2026-11-04 | [UI.md](UI.md#ctl-001-mouse-controls) |
| CURSOR-000-migrate-agent-to-cursor | Moved `.agent/` into `.cursor/` | 2026-09-08 | [ART.md](ART.md#cursor-000-migrate-agent-to-cursor) |
| DEF-0-combat-package | The combat package and the combat domain | 2026-10-29 | [DEF.md](DEF.md#def-0-combat-package) |
| DEF-1-projectiles-armor | Projectiles, burst, armor, combat overlays | 2026-10-29 | [DEF.md](DEF.md#def-1-projectiles-armor) |
| DEF-2-weapons-armor-explosions | Weapons, armor, explosions | 2026-10-29 | [DEF.md](DEF.md#def-2-weapons-armor-explosions) |
| DEF-3-cover-traps-turrets-mortars-doors | Cover, traps, turrets, mortars and doors | 2026-10-29 | [DEF.md](DEF.md#def-3-cover-traps-turrets-mortars-doors) |
| DEF-4-R1-raids-drafted-control | Raids, drafted control, IED leftovers, combat log | 2026-10-29 | [DEF.md](DEF.md#def-4-r1-raids-drafted-control) |
| DEF-4-R2-mechanoids-ship-part | Mechanoids and the crashed ship part | 2026-10-29 | [DEF.md](DEF.md#def-4-r2-mechanoids-ship-part) |
| DEPS-001-libraries-and-rules | Libraries at the edge, and cleaning up what we wrote by hand | 2026-11-03 | [OPS.md](OPS.md#deps-001-libraries-and-rules) |
| DOCS-001-readme-screenshots | README with screenshots and clips | 2026-10-19 | [OPS.md](OPS.md#docs-001-readme-screenshots) |
| DOCS-002-readme-media-refresh | README "See it move" media re-captured | 2026-10-26 | [OPS.md](OPS.md#docs-002-readme-media-refresh) |
| DOCS-003-readme-rework | README that shows the game first | 2026-11-05 | [OPS.md](OPS.md#docs-003-readme-rework) |
| DOS-0A-art | Colonist card colours and icons | 2026-11-03 | [DOS.md](DOS.md#dos-0a-art) |
| DOS-1-card | The pawn card's data | 2026-11-03 | [DOS.md](DOS.md#dos-1-card) |
| DOS-2-sheet | `sheet`, the card's pure logic | 2026-11-03 | [DOS.md](DOS.md#dos-2-sheet) |
| DOS-3-panel | `dossier.Panel` | 2026-11-04 | [DOS.md](DOS.md#dos-3-panel) |
| DOS-4-pane | The card in the game | 2026-11-04 | [DOS.md](DOS.md#dos-4-pane) |
| DOS-5-setup | The card's setup dialog | 2026-11-04 | [DOS.md](DOS.md#dos-5-setup) |
| DOS-6-editor | Sections tab, Presets tab, preset store, folder opener | 2026-11-04 | [DOS.md](DOS.md#dos-6-editor) |
| DOS-7-letters | Letters and the floating card | 2026-11-04 | [DOS.md](DOS.md#dos-7-letters) |
| DOS-8-lessons | Colonist card lessons (RHUD-043..047) | 2026-11-04 | [DOS.md](DOS.md#dos-8-lessons) |
| DOS-9-card-on | The colonist card on by default, and the audit | 2026-11-04 | [DOS.md](DOS.md#dos-9-card-on) |
| DOS-10-compact | The docked card in 30 % of the screen | 2026-11-04 | [DOS.md](DOS.md#dos-10-compact) |
| ENG-001-scale-500 | Engine scale: 200 to 500 tiles a side | 2026-10-22 | [ENG.md](ENG.md#eng-001-scale-500) |
| ENG-002-grass-batch | Grass drawn as composed tufts (PLAY-010) | 2026-10-23 | [ENG.md](ENG.md#eng-002-grass-batch) |
| ENG-003-social-contract | Social system contract, M0 (SOCIAL_SYSTEM_CONTRACT C1–C4) | 2026-10-23 | [ENG.md](ENG.md#eng-003-social-contract) |
| ENG-004-play019-ram-graphics-crops | RAM-aware graphics defaults, memory diet, crops on their tile (PLAY-019) | 2026-10-25 | [ENG.md](ENG.md#eng-004-play019-ram-graphics-crops) |
| ENG-005-play021-electricity | Electricity, as RimWorld does it (PLAY-021) | 2026-10-26 | [ENG.md](ENG.md#eng-005-play021-electricity) |
| ENG-006-play024-trade-turrets | Sentry turrets, trade, radio console (PLAY-024 phases 0–3) | 2026-10-26 | [ENG.md](ENG.md#eng-006-play024-trade-turrets) |
| ENG-007-play024-military-aid | Military aid by radio (PLAY-024 phase 4) | 2026-10-26 | [ENG.md](ENG.md#eng-007-play024-military-aid) |
| ENG-008-plans-include-parallel-work | Every plan includes its parallel work | 2026-10-29 | [ENG.md](ENG.md#eng-008-plans-include-parallel-work) |
| ENG-009-agent-cleanup | Parallel work reuses and cleans up agents | 2026-10-30 | [ENG.md](ENG.md#eng-009-agent-cleanup) |
| FEED-001-tame-animal-food | Hungry tame animals reach the food the colony has | 2026-10-30 | [FOOD.md](FOOD.md#feed-001-tame-animal-food) |
| FISH-tanks | Fish tanks | 2026-11-02 | [FOOD.md](FOOD.md#fish-tanks) |
| FISH_TANK_PLAN | Fish tanks and the MAP-5 fixes: gap analysis and plan (FISH-*, MAP-5, STK-AL 2) | 2026-11-02 | [FOOD.md](FOOD.md#fish_tank_plan) |
| FOOD-0-gap-analysis-and-contract | Food production gap analysis and contract | 2026-10-30 | [FOOD.md](FOOD.md#food-0-gap-analysis-and-contract) |
| FOOD-1-nutrition-and-hunger | Nutrition and hunger at RimWorld fidelity | 2026-10-30 | [FOOD.md](FOOD.md#food-1-nutrition-and-hunger) |
| FOOD-2-growing | Growing at RimWorld fidelity | 2026-10-30 | [FOOD.md](FOOD.md#food-2-growing) |
| FOOD-3-animals-butchering | Animals, butchering and hunting at RimWorld fidelity | 2026-10-30 | [FOOD.md](FOOD.md#food-3-animals-butchering) |
| FOOD-4-cooking-eating | Cooking, preservation and eating | 2026-10-31 | [FOOD.md](FOOD.md#food-4-cooking-eating) |
| FOOD-5-food-ui | Food UI | 2026-10-31 | [FOOD.md](FOOD.md#food-5-food-ui) |
| FOOD-6-close-out | FOOD close-out | 2026-10-31 | [FOOD.md](FOOD.md#food-6-close-out) |
| GEAR-0-contract | Contract for gear, outfits, options and ailments | 2026-11-06 | [GEAR.md](GEAR.md#gear-0-contract) |
| GEAR-1-gear-actions | Drop, Eat and Lock in the simulation | 2026-11-06 | [GEAR.md](GEAR.md#gear-1-gear-actions) |
| GEAR-1-gear-facts | Gear tab summary numbers; armour slows the wearer | 2026-11-06 | [GEAR.md](GEAR.md#gear-1-gear-facts) |
| GEAR-2-gear-tab | The Gear tab as a full equipment screen | 2026-11-06 | [GEAR.md](GEAR.md#gear-2-gear-tab) |
| GEAR-2-infocard | The read-only Info window | 2026-11-06 | [GEAR.md](GEAR.md#gear-2-infocard) |
| GEAR-3-outfits-sim | Apparel policies, simulation side | 2026-11-06 | [GEAR.md](GEAR.md#gear-3-outfits-sim) |
| GEAR-3-outfits-ui | Apparel policies, Assign window | 2026-11-06 | [GEAR.md](GEAR.md#gear-3-outfits-ui) |
| HAUL-001-batched-delivery | Batched construction delivery | 2026-10-21 | [STORAGE.md](STORAGE.md#haul-001-batched-delivery) |
| HAUL-002-gather-nearby-piles | Haulers gather nearby piles before delivering | 2026-10-30 | [STORAGE.md](STORAGE.md#haul-002-gather-nearby-piles) |
| HAUL-003-harvest-sweep | Harvest the field before hauling its piles | 2026-11-04 | [STORAGE.md](STORAGE.md#haul-003-harvest-sweep) |
| LOG-001-creature-event-log | Creature event log | 2026-10-21 | [OPS.md](OPS.md#log-001-creature-event-log) |
| MAP-0-contract | Multiple maps, gap analysis and inert contract | 2026-11-02 | [WORLD.md](WORLD.md#map-0-contract) |
| MAP-1-site-maps | Site maps, ticking maps, switching | 2026-11-02 | [WORLD.md](WORLD.md#map-1-site-maps) |
| MAP-2-attack-and-pods | Attacking settlements and pod modes | 2026-11-02 | [WORLD.md](WORLD.md#map-2-attack-and-pods) |
| MAP-3-founding-and-encounters | Founding, abandoning, caravan speed and encounters | 2026-11-02 | [WORLD.md](WORLD.md#map-3-founding-and-encounters) |
| MAP-4-endings-turrets-neutral-attacks | The everyone-gone ending, hostile turrets, neutral attacks | 2026-11-02 | [WORLD.md](WORLD.md#map-4-endings-turrets-neutral-attacks) |
| MAP-5-doors-visitors-wanderers | Enemy doors, hostile visitors, new wanderers | 2026-11-02 | [WORLD.md](WORLD.md#map-5-doors-visitors-wanderers) |
| MAP-6-deferred-four | The four deferred items, second pass | 2026-11-03 | [WORLD.md](WORLD.md#map-6-deferred-four) |
| MED-00-contract | Medical overhaul contract and save groundwork | 2026-10-26 | [MED.md](MED.md#med-00-contract) |
| MED-01-bleeding | Bleeding, blood loss, pain, downed | 2026-10-26 | [MED.md](MED.md#med-01-bleeding) |
| MED-02-tend-immunity | Tend quality, healing, scars, infection vs immunity | 2026-10-26 | [MED.md](MED.md#med-02-tend-immunity) |
| MED-03-care-policy | Medical care policy, doctor work order, emergency tend | 2026-10-26 | [MED.md](MED.md#med-03-care-policy) |
| MED-04-diseases | Diseases and outbreaks | 2026-10-26 | [MED.md](MED.md#med-04-diseases) |
| MED-05-anatomy-bionics | Organs, capacities, bionic tier | 2026-10-26 | [MED.md](MED.md#med-05-anatomy-bionics) |
| MED-06-surgery | Surgery v2, organs, euthanasia, anesthetic, vitals monitor | 2026-10-26 | [MED.md](MED.md#med-06-surgery) |
| MED-07-drug-health | Drug health effects (painkiller, prophylaxis, overdose) | 2026-10-26 | [MED.md](MED.md#med-07-drug-health) |
| MED-08-aging | Aging and chronic conditions | 2026-10-26 | [MED.md](MED.md#med-08-aging) |
| MED-09-animals | Animal medicine | 2026-10-26 | [MED.md](MED.md#med-09-animals) |
| MED-10-capacity-tooltip-monitor-gate | Capacity-breakdown tooltip, vitals monitor research gate | 2026-10-26 | [MED.md](MED.md#med-10-capacity-tooltip-monitor-gate) |
| MED-11-food-poisoning-caravans | Food poisoning, disease on caravans | 2026-10-26 | [MED.md](MED.md#med-11-food-poisoning-caravans) |
| MED-12-downed-animals | Downed animals | 2026-10-26 | [MED.md](MED.md#med-12-downed-animals) |
| MED-13-animal-anatomy | Per-archetype animal anatomy | 2026-10-26 | [MED.md](MED.md#med-13-animal-anatomy) |
| MOD-000-mods-programme | RimHUD, Neat Storage and Common Sense programme | 2026-11-03 | [MOD.md](MOD.md#mod-000-mods-programme) |
| MOD-W0-P-budget | Room in package `game` | 2026-11-03 | [MOD.md](MOD.md#mod-w0-p-budget) |
| NG1_NEW_GAME_PLAN | NG-1: complete new-game flow, big maps at 60 fps, generated colonists, factions, water | 2026-10-22 | [NG.md](NG.md#ng1_new_game_plan) |
| NST-A-kinds | Neat Storage kinds (domain) | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-a-kinds) |
| NST-B-storage | Storage behaviour of the 27 Neat Storage pieces | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-b-storage) |
| NST-C-architect | Architect category, cards and inspector for Neat Storage | 2026-11-04 | [STORAGE.md](STORAGE.md#nst-c-architect) |
| NST-D-renderer | Drawing stored stacks in the Neat Storage pieces | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-d-renderer) |
| NST-E1-art | Neat Storage art: display furniture | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-e1-art) |
| NST-E2-art | Neat Storage racks art | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-e2-art) |
| NST-E3-art | Neat Storage bulk art | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-e3-art) |
| NST-G-admire | recreate.Admire | 2026-11-03 | [STORAGE.md](STORAGE.md#nst-g-admire) |
| OPT-001-architect-orders-text | Architect and orders text through the catalog | 2026-11-06 | [PERF.md](PERF.md#opt-001-architect-orders-text) |
| OPT-1-camera-pause | Camera, pause and window options | 2026-11-06 | [PERF.md](PERF.md#opt-1-camera-pause) |
| OPT-001-dossier-text | Dossier text through the catalog (W3-DOSS) | 2026-11-06 | [PERF.md](PERF.md#opt-001-dossier-text) |
| OPT-1-hud | HUD: clock and colonist bar options | 2026-11-06 | [PERF.md](PERF.md#opt-1-hud) |
| OPT-1-kit | Kit: tooltip delay, colour vision, larger small text, confirmations | 2026-11-06 | [PERF.md](PERF.md#opt-1-kit) |
| OPT-1-lang | Translation catalog and its tools (OPT-001 infrastructure) | 2026-11-06 | [PERF.md](PERF.md#opt-1-lang) |
| OPT-1-map-options | Map-side Options rows (OPT-010, 014, 016, 026) | 2026-11-06 | [PERF.md](PERF.md#opt-1-map-options) |
| OPT-1-misc | Wizard, main menu, Architect and developer-tool options | 2026-11-06 | [PERF.md](PERF.md#opt-1-misc) |
| OPT-1-options-rows | Options rows (W1-OPTS) | 2026-11-06 | [PERF.md](PERF.md#opt-1-options-rows) |
| OPT-001-w3-hud-learn-dialogs | Wave 3: hud, learn and dialogs text through the catalog | 2026-11-06 | [PERF.md](PERF.md#opt-001-w3-hud-learn-dialogs) |
| OPT-001-w3-root-options-uikit-overlay-text | Wave 3: root, options, uikit, render/overlay text | 2026-11-06 | [PERF.md](PERF.md#opt-001-w3-root-options-uikit-overlay-text) |
| OPT-001-w3-windows-text | Wave 3: colony windows' text | 2026-11-06 | [PERF.md](PERF.md#opt-001-w3-windows-text) |
| OPT-001-w4-simtext | Wave 4 (W4-SIMTEXT): sim messages and letters carry their format | 2026-11-06 | [PERF.md](PERF.md#opt-001-w4-simtext) |
| OPT-001-wave3-integration | Wave 3: every UI string through the catalog (lead integration) | 2026-11-06 | [PERF.md](PERF.md#opt-001-wave3-integration) |
| OPT-001-wave3-wizard-menu-world | Wave 3: wizard, main menu, world view, world info, globe text | 2026-11-06 | [PERF.md](PERF.md#opt-001-wave3-wizard-menu-world) |
| OPT-2-art | Art: Gear tab icons, the game's pointer, colour-vision palettes | 2026-11-06 | [PERF.md](PERF.md#opt-2-art) |
| OPT-2-cursor | OPT-028 custom cursor takes effect | 2026-11-06 | [PERF.md](PERF.md#opt-2-cursor) |
| OPT-2-journeys | Options and camera journeys (W2-JOURNEY) | 2026-11-06 | [PERF.md](PERF.md#opt-2-journeys) |
| OPT-4-lang-hud | Messages and letters drawn through the catalog (client half of Wave 4) | 2026-11-06 | [PERF.md](PERF.md#opt-4-lang-hud) |
| PERF-001-tickgrown-regression | `BenchmarkTickGrown` regression after ENG-003 | 2026-10-24 | [PERF.md](PERF.md#perf-001-tickgrown-regression) |
| PERF-002-120fps-one-percent-low | The 1 % low on 500x500 maps (issue #6) | 2026-10-27 | [PERF.md](PERF.md#perf-002-120fps-one-percent-low) |
| PERF-003-render-cap | The render cap (Mac kernel panics of 2026-11-04) | 2026-11-04 | [PERF.md](PERF.md#perf-003-render-cap) |
| PERF-004-gar-baseline | The VSync-on probe matrix baseline (GAR C3) | 2026-11-08 | [PERF.md](PERF.md#perf-004-gar-baseline) |
| PERF-005-lod-by-cells | The LOD tiers follow the cells in view (GAR-155) | 2026-11-08 | [PERF.md](PERF.md#perf-005-lod-by-cells) |
| PERF-006-frame-pacing | Walkers move the same distance every frame: a paced animation clock (GAR-091) | 2026-11-08 | [PERF.md](PERF.md#perf-006-frame-pacing) |
| PERF-007-draw-measured | Draw CPU measured before building E7; the pixel-diff harness | 2026-11-08 | [PERF.md](PERF.md#perf-007-draw-measured) |
| PERF-008-aa-shapes | Anti-aliased shapes are feathered triangles, not Ebitengine vector paths | 2026-11-08 | [PERF.md](PERF.md#perf-008-aa-shapes) |
| PERF-009-render-pass-order | Render draw order, prepare pass and frame constants (render-b) | 2026-11-08 | [PERF.md](PERF.md#perf-009-render-pass-order) |
| PERF-010-async-autosave | Autosaves write on a worker goroutine (GAR-190) | 2026-11-08 | [PERF.md](PERF.md#perf-010-async-autosave) |
| PERF-011-periodic-spread | The day, hour and 250-tick work no longer fires all at once (GAR-006..009) | 2026-11-08 | [PERF.md](PERF.md#perf-011-periodic-spread) |
| PERF-012-sim-index-fixes | Serial index fixes before any parallel stage (sim S1–S6, S4b) | 2026-11-08 | [PERF.md](PERF.md#perf-012-sim-index-fixes) |
| PERF-013-round2-acceptance | In-game acceptance of GAR Part E round 2 (fps matrix, VSync on, quiet machine) | 2026-11-09 | [PERF.md](PERF.md#perf-013-round2-acceptance) |
| PERF-014-ground-batch-and-stack-text | GAR-159 ground tiles batched into one lit call; GAR-160 stack-count text cached as one quad per count | 2026-11-09 | [PERF.md](PERF.md#perf-014-ground-batch-and-stack-text) |
| PLAY010_PLAN | PLAY-010 play-feedback batch (14 items, up to 15 agents) | 2026-10-23 | [PLAY.md](PLAY.md#play010_plan) |
| PLAY011_PLAN | PLAY-011 auto-oriented doors and gates, in-place material swaps, unmovable fences | 2026-10-24 | [PLAY.md](PLAY.md#play011_plan) |
| PLAY012_PLAN | PLAY-012 Plan tool and crisp UI scaling (Auto + presets, HiDPI) | 2026-10-24 | [PLAY.md](PLAY.md#play012_plan) |
| PLAY013_PLAN | MAPGEN-2 field-based local map generator, debug views, fog of war (file is PLAY013_PLAN) | 2026-10-24 | [PLAY.md](PLAY.md#play013_plan) |
| PLAY014_PLAN | MAPGEN-3 cave plants, road re-routing, bridges, sealed vaults (file is PLAY014_PLAN) | 2026-10-24 | [PLAY.md](PLAY.md#play014_plan) |
| PLAY015_PLAN | PLAY-015 areas, pens, Home, snow clearing and fire | 2026-10-24 | [PLAY.md](PLAY.md#play015_plan) |
| PLAY016_PLAN | PLAY-016 crops redone in the grass and bush style, plus visible cold death | 2026-10-25 | [PLAY.md](PLAY.md#play016_plan) |
| PLAY017_PLAN | PLAY-017 plan: sowing, hauling, outfits, animals, plants and light, chase warnings, UI-scale fixes, visual tests | 2026-10-25 | [PLAY.md](PLAY.md#play017_plan) |
| PLAY018_PLAN | PLAY-018 plan: colony HUD re-layout, command buttons, charcoal palette, letters, Learning Helper | 2026-10-25 | [PLAY.md](PLAY.md#play018_plan) |
| PLAY019_PLAN | PLAY-019 plan: RAM-aware parallel work, hardware-aware graphics defaults, crops on their own tile | 2026-10-25 | [PLAY.md](PLAY.md#play019_plan) |
| PLAY020_PLAN | PLAY-020 plan: bubbles, zone fill, lamp-lit rooms, sowing, bed rest, medical beds, zone management | 2026-10-25 | [PLAY.md](PLAY.md#play020_plan) |
| PLAY021_PLAN | PLAY-021 plan: electricity, RimWorld-style | 2026-10-25 | [PLAY.md](PLAY.md#play021_plan) |
| PLAY022_PLAN | PLAY-022 plan: bigger animals, natural animal motion, puddles that blend | 2026-10-27 | [PLAY.md](PLAY.md#play022_plan) |
| PLAY024_PLAN | PLAY-024 plan: sentry turret, trade, radio console | 2026-10-26 | [PLAY.md](PLAY.md#play024_plan) |
| PLAY-001-path-jobs-names | Pawn names, A* pathing and the job FSM | 2026-09-16 | [PLAY.md](PLAY.md#play-001-path-jobs-names) |
| PLAY-002-eat-sleep-needs | Eat, sleep and needs | 2026-09-16 | [PLAY.md](PLAY.md#play-002-eat-sleep-needs) |
| PLAY-003-stockpile-haul-architect | First playable: stockpile, haul, build | 2026-09-16 | [PLAY.md](PLAY.md#play-003-stockpile-haul-architect) |
| PLAY-004-architect-and-pawn-orders | Build menu, drafting, right-click orders | 2026-10-05 | [PLAY.md](PLAY.md#play-004-architect-and-pawn-orders) |
| PLAY-005-world-resources | Map generation, plants, mining, stone | 2026-10-05 | [PLAY.md](PLAY.md#play-005-world-resources) |
| PLAY-006-food-production | Food kinds and growing zones | 2026-10-05 | [PLAY.md](PLAY.md#play-006-food-production) |
| PLAY-007-benches-and-bills | Multi-tile buildings, benches, bills | 2026-10-05 | [PLAY.md](PLAY.md#play-007-benches-and-bills) |
| PLAY-008-rooms-mood-lighting | Rooms, impressiveness, mood, lamp light | 2026-10-05 | [PLAY.md](PLAY.md#play-008-rooms-mood-lighting) |
| PLAY-009-combat-animals-hunting | Health, animals, weapons, combat, hunting | 2026-10-05 | [PLAY.md](PLAY.md#play-009-combat-animals-hunting) |
| PLAY-010-rescue-butchering-alerts | Rescue, corpses, butchering, alerts | 2026-10-06 | [PLAY.md](PLAY.md#play-010-rescue-butchering-alerts) |
| PLAY-011-sleeping-spots | Sleeping spot replaces the cot | 2026-10-06 | [PLAY.md](PLAY.md#play-011-sleeping-spots) |
| PLAY-012-double-bed-stacks-crops | Double bed, big stacks, seven real crops | 2026-10-06 | [PLAY.md](PLAY.md#play-012-double-bed-stacks-crops) |
| PLAY-013-sleep-and-anti-flap | Sleep rules, anti-flap pass, zones over floors | 2026-10-21 | [PLAY.md](PLAY.md#play-013-sleep-and-anti-flap) |
| PLAY-022-power-followups | Lamp bulb, solar weather, smooth lamp light | 2026-10-26 | [PLAY.md](PLAY.md#play-022-power-followups) |
| PLAY-023-zones-rimworld-rules | Zone placement rules, dumping stockpile, storage priority | 2026-10-26 | [PLAY.md](PLAY.md#play-023-zones-rimworld-rules) |
| PRD-0-craft-package | The craft package and the quality domain | 2026-10-29 | [BASE.md](BASE.md#prd-0-craft-package) |
| PRD-1-bill-settings | Bill settings, multi-ingredient recipes, gathering | 2026-10-29 | [BASE.md](BASE.md#prd-1-bill-settings) |
| PRD-2-quality-stuff-condition | Quality, stuff, condition, inspiration | 2026-10-29 | [BASE.md](BASE.md#prd-2-quality-stuff-condition) |
| PRD-3-benches-work-speed-sculptures | Benches, work speed, art and sculptures | 2026-10-29 | [BASE.md](BASE.md#prd-3-benches-work-speed-sculptures) |
| PRD-4-catalogue-stockpiles-shelves | Recipe catalogue, stockpile filters, shelves, barrel | 2026-10-29 | [BASE.md](BASE.md#prd-4-catalogue-stockpiles-shelves) |
| PWR-power-wires | Visible power wires and Reconnect | 2026-11-03 | [BASE.md](BASE.md#pwr-power-wires) |
| QA-001-player-journeys | Player-journey tests, CI on every push, real-path clicks | 2026-10-27 | [OPS.md](OPS.md#qa-001-player-journeys) |
| QST-1-quests-contract | Quests contract: domain, root, glue | 2026-11-07 | [WORLD.md](WORLD.md#qst-1-quests-contract) |
| QST-2a-quest-kinds-without-sites | Joiner, ore lump, trade request, peace talks; the wanderer letter | 2026-11-07 | [WORLD.md](WORLD.md#qst-2a-quest-kinds-without-sites) |
| QST-2b-quest-sites | Site maps for quests; stash, refugee, camp and rescue kinds; ore-site threats | 2026-11-07 | [WORLD.md](WORLD.md#qst-2b-quest-sites) |
| QST-3-quests-ui | Quests tab and window, quest letters and alert, quest sites on the globe | 2026-11-07 | [WORLD.md](WORLD.md#qst-3-quests-ui) |
| QST-4-quests-verification | The quest soak and replay checks | 2026-11-07 | [WORLD.md](WORLD.md#qst-4-quests-verification) |
| QST-5-quests-close-out | The last open quest rows: offer help, site threats, reachability, rare rewards | 2026-11-07 | [WORLD.md](WORLD.md#qst-5-quests-close-out) |
| QST-gap-analysis | Quests gap analysis | 2026-11-07 | [WORLD.md](WORLD.md#qst-gap-analysis) |
| REF-1-prioritize-refuel-auto-toggle | Ordered refuels and the auto refuel switch | 2026-11-06 | [BASE.md](BASE.md#ref-1-prioritize-refuel-auto-toggle) |
| REL-001-release-workflow | Release builds on tags | 2026-10-21 | [OPS.md](OPS.md#rel-001-release-workflow) |
| REL-002-goreleaser | GoReleaser publishes the release | 2026-11-02 | [OPS.md](OPS.md#rel-002-goreleaser) |
| RES-1-reservations | Reservations, RimWorld-style | 2026-10-30 | [BASE.md](BASE.md#res-1-reservations) |
| REVIEW-001-playable-survival | Quality pass on PLAY-001 through SURV-001 | 2026-09-16 | [OPS.md](OPS.md#review-001-playable-survival) |
| RSCH-0-gap-analysis-and-contract | Research gap analysis and contract | 2026-11-01 | [BASE.md](BASE.md#rsch-0-gap-analysis-and-contract) |
| RSCH-1-research-simulation | Research simulation at wiki fidelity | 2026-11-01 | [BASE.md](BASE.md#rsch-1-research-simulation) |
| RSCH-2-research-tab | The Research tab as RimWorld's research screen | 2026-11-01 | [BASE.md](BASE.md#rsch-2-research-tab) |
| RSCH-3-research-bench-art | Research bench and multi-analyzer art | 2026-11-01 | [BASE.md](BASE.md#rsch-3-research-bench-art) |
| SHADOW-creatures-cast-none | Creatures cast no shadow (agent SH) | 2026-11-01 | [ART.md](ART.md#shadow-creatures-cast-none) |
| SHT-MIN-RSC-0-gap-analyses | Gap analyses of shooting, mining and rescue | 2026-11-05 | [DEF.md](DEF.md#sht-min-rsc-0-gap-analyses) |
| SIM-001-climate-and-spoilage | Climate, seasons, spoilage and frost | 2026-10-06 | [SIM.md](SIM.md#sim-001-climate-and-spoilage) |
| SIM-002-drying-rack | Drying rack: dried food that never rots | 2026-10-06 | [SIM.md](SIM.md#sim-002-drying-rack) |
| SIM-003-heat-and-exposure | Room heat, campfires and exposure | 2026-10-06 | [SIM.md](SIM.md#sim-003-heat-and-exposure) |
| SIM-004-free-time-and-bedtime | Free time, bedtime priority, stack counts | 2026-10-06 | [SIM.md](SIM.md#sim-004-free-time-and-bedtime) |
| SIM-005-clothing | Clothing | 2026-10-06 | [SIM.md](SIM.md#sim-005-clothing) |
| SIM-006-cooking-and-dining | Campfire cooking and eating at tables | 2026-10-06 | [SIM.md](SIM.md#sim-006-cooking-and-dining) |
| SIM-007-recreation-and-mental-breaks | Recreation need and mental breaks | 2026-10-06 | [SIM.md](SIM.md#sim-007-recreation-and-mental-breaks) |
| SIM-008-threats | Threats: raids and winter wolf packs | 2026-10-06 | [SIM.md](SIM.md#sim-008-threats) |
| SIM-009-defences | Defences: doors, sandbags, spike traps | 2026-10-06 | [SIM.md](SIM.md#sim-009-defences) |
| SIM-010-infection-and-medicine | Infection, herbal medicine, healroot | 2026-10-06 | [SIM.md](SIM.md#sim-010-infection-and-medicine) |
| SIM-011-research | Research | 2026-10-06 | [SIM.md](SIM.md#sim-011-research) |
| SIM-012-animal-handling | Animal handling: taming, pens, guards, manhunters | 2026-10-06 | [SIM.md](SIM.md#sim-012-animal-handling) |
| SIM-013-raid-scaling-sieges-sappers | Wealth-scaled raids, sieges and sappers | 2026-10-06 | [SIM.md](SIM.md#sim-013-raid-scaling-sieges-sappers) |
| SIM-014-movement-timing-and-blocking | Movement timing, diagonal cost and creature blocking | 2026-10-19 | [SIM.md](SIM.md#sim-014-movement-timing-and-blocking) |
| SIM-015-action-read-model-knife-spear | Action read model, knife and spear | 2026-10-19 | [SIM.md](SIM.md#sim-015-action-read-model-knife-spear) |
| SIM-016-weather | Weather | 2026-10-19 | [SIM.md](SIM.md#sim-016-weather) |
| SIM-017-burial-cremation | Corpses, graves, funeral pyres and mourning | 2026-10-19 | [SIM.md](SIM.md#sim-017-burial-cremation) |
| SIM-018-roofs | Roofs | 2026-10-20 | [SIM.md](SIM.md#sim-018-roofs) |
| SIM-019-wildlife | Wildlife: 61 species | 2026-10-21 | [SIM.md](SIM.md#sim-019-wildlife) |
| SIM-020-blood-stains | Blood and ichor: stains on floors and bodies | 2026-10-21 | [SIM.md](SIM.md#sim-020-blood-stains) |
| SIM-021-dirt-cleaning-home-carpets | Tracked dirt, cleaning, Home area, cotton, carpets | 2026-10-22 | [SIM.md](SIM.md#sim-021-dirt-cleaning-home-carpets) |
| SIM-023-biomes | Biomes: twelve biomes, new ground and wild plants | 2026-10-22 | [SIM.md](SIM.md#sim-023-biomes) |
| SIM-024-animal-ecology | Animal ecology: needs, states, grazing, troughs | 2026-10-22 | [SIM.md](SIM.md#sim-024-animal-ecology) |
| SIM-025-wildlife-population-events | Wildlife population and nature events | 2026-10-22 | [SIM.md](SIM.md#sim-025-wildlife-population-events) |
| SIM-026-animal-breeding-products | Animal breeding, life stages and products | 2026-10-22 | [SIM.md](SIM.md#sim-026-animal-breeding-products) |
| SIM-027-postpone-blocked-work | Postponing blocked and unreachable work | 2026-10-22 | [SIM.md](SIM.md#sim-027-postpone-blocked-work) |
| SIM-028-floors-under-furniture-fences-forbid | Floors under furniture, fences and gates, forbid | 2026-10-22 | [SIM.md](SIM.md#sim-028-floors-under-furniture-fences-forbid) |
| SIM-029-stone-types-ore-smoothing-stonecutting | Stone types, ore veins, smoothing and stonecutting | 2026-10-22 | [SIM.md](SIM.md#sim-029-stone-types-ore-smoothing-stonecutting) |
| SIM-030-planet-factions | Planet generator, landing-site parameters and factions (NG-1) | 2026-10-22 | [SIM.md](SIM.md#sim-030-planet-factions) |
| SIM-031-storyteller-difficulty | Storytellers, difficulty and threat points (NG-1) | 2026-10-22 | [SIM.md](SIM.md#sim-031-storyteller-difficulty) |
| SIM-032-pawn-identity-skills | Colonist identity, skills in play and character creation (NG-1) | 2026-10-22 | [SIM.md](SIM.md#sim-032-pawn-identity-skills) |
| SIM-033-water-sites | Landing-site maps and water (NG-1) | 2026-10-22 | [SIM.md](SIM.md#sim-033-water-sites) |
| SIM-034-components-medicine | Components, industrial medicine, tool cabinet, hospital bed, pod shells (NG-1) | 2026-10-22 | [SIM.md](SIM.md#sim-034-components-medicine) |
| SIM-035-animal-reactions | Animal reactions to being hit and predator charges (PLAY-010) | 2026-10-23 | [SIM.md](SIM.md#sim-035-animal-reactions) |
| SIM-036-colonist-flee | Unarmed colonists flee animals (PLAY-010) | 2026-10-23 | [SIM.md](SIM.md#sim-036-colonist-flee) |
| SIM-037-walk-through | Colonists and tame animals walk through each other (PLAY-010) | 2026-10-23 | [SIM.md](SIM.md#sim-037-walk-through) |
| SIM-038-wild-plants-grass-growth | Seven wild ground plants and growing grass (PLAY-010) | 2026-10-23 | [SIM.md](SIM.md#sim-038-wild-plants-grass-growth) |
| SIM-039-body-surgery | Body parts, wounds, prosthetics and surgery (ENG-003 M1) | 2026-10-24 | [SIM.md](SIM.md#sim-039-body-surgery) |
| SIM-040-drugs | Drugs, tolerance, addiction and policies (ENG-003 M2) | 2026-10-24 | [SIM.md](SIM.md#sim-040-drugs) |
| SIM-041-social-core | People registry, family graph, opinions, interactions, memories, grief (ENG-003 M3+M7) | 2026-10-24 | [SIM.md](SIM.md#sim-041-social-core) |
| SIM-042-romance-weddings | Romance, weddings, affairs, breakups, partner beds (ENG-003 M5) | 2026-10-24 | [SIM.md](SIM.md#sim-042-romance-weddings) |
| SIM-043-social-fights | Social fights (ENG-003 M6) | 2026-10-24 | [SIM.md](SIM.md#sim-043-social-fights) |
| SIM-045-prisoners | Prisoners: capture, life, warden work, recruit, release, execute (ENG-003 M8) | 2026-10-24 | [SIM.md](SIM.md#sim-045-prisoners) |
| SIM-046-world-caravans-areas | Runtime world, caravans, visitors, allowed areas (ENG-003 M9a) | 2026-10-24 | [SIM.md](SIM.md#sim-046-world-caravans-areas) |
| SIM-047-animal-bonds | Animal bonds, masters, training, veterinary care (ENG-003 M10) | 2026-10-24 | [SIM.md](SIM.md#sim-047-animal-bonds) |
| SIM-048-social-polish | Social Impact scaling, grief labels, guard/release, letters (ENG-003 polish) | 2026-10-24 | [SIM.md](SIM.md#sim-048-social-polish) |
| SIM-049-usable-surgery | Usable surgery: replace healthy parts, new implants, waiting reasons | 2026-10-24 | [SIM.md](SIM.md#sim-049-usable-surgery) |
| SIM-050-mapgen-pipeline | Staged field-based map generator (PLAY-013, MAPGEN-2) | 2026-10-24 | [SIM.md](SIM.md#sim-050-mapgen-pipeline) |
| SIM-051-fog-of-war | Fog of war (PLAY-013, MAPGEN-2 §5) | 2026-10-24 | [SIM.md](SIM.md#sim-051-fog-of-war) |
| SIM-052-cave-plants-light | Simulation light level for plants; duskmoss (PLAY-014, MAPGEN-3 §1) | 2026-10-24 | [SIM.md](SIM.md#sim-052-cave-plants-light) |
| SIM-053-roads-bridges | Road bridges (mapgen) and player bridges (PLAY-014, MAPGEN-3 §2-3) | 2026-10-24 | [SIM.md](SIM.md#sim-053-roads-bridges) |
| SIM-054-sealed-vaults | Sealed ancient vaults (PLAY-014, MAPGEN-3 §4) | 2026-10-24 | [SIM.md](SIM.md#sim-054-sealed-vaults) |
| SIM-055-allowed-areas | Allowed areas rebuilt to the user's spec (PLAY-015 §1) | 2026-10-24 | [SIM.md](SIM.md#sim-055-allowed-areas) |
| SIM-056-pens | Pens as markers with enclosure, roping and settings (PLAY-015 §2) | 2026-10-24 | [SIM.md](SIM.md#sim-056-pens) |
| SIM-057-home-snow | Home auto-expansion and per-tile snow (PLAY-015 §3-4) | 2026-10-24 | [SIM.md](SIM.md#sim-057-home-snow) |
| SIM-058-fire | Fire: spread, damage, firefighting, danger (PLAY-015 §5) | 2026-10-24 | [SIM.md](SIM.md#sim-058-fire) |
| SIM-059-plant-damage | Gradual cold/heat plant damage (PLAY-016 §3) | 2026-10-25 | [SIM.md](SIM.md#sim-059-plant-damage) |
| SIM-060-play017-colony-rules | Colony rules: sowing, light, hauling claims, apparel, tame feeding, chase warnings (PLAY-017) | 2026-10-25 | [SIM.md](SIM.md#sim-060-play017-colony-rules) |
| SIM-061-bench-before | Benchmark baseline before the care package (record) | 2026-10-29 | [SIM.md](SIM.md#sim-061-bench-before) |
| SIM-061-care-package | Care package, sub-package rule, pawn-interaction gap analysis (SIM-061…066 programme) | 2026-10-29 | [SIM.md](SIM.md#sim-061-care-package) |
| SIM-062-pawn-orders | Orders on a pawn: rescue, capture, arrest, prioritize tending | 2026-10-29 | [SIM.md](SIM.md#sim-062-pawn-orders) |
| SIM-063-rescue-fidelity | Rescue fidelity, safe beds, guests, prisoners back to bed | 2026-10-29 | [SIM.md](SIM.md#sim-063-rescue-fidelity) |
| SIM-064-feeding-patients | Feeding patients, downed hunger | 2026-10-29 | [SIM.md](SIM.md#sim-064-feeding-patients) |
| SIM-065-visit-sick-strip | Visiting the sick, strip | 2026-10-29 | [SIM.md](SIM.md#sim-065-visit-sick-strip) |
| SIM-066-mental-breaks | Mental breaks that target others; W3 integration (SIM-063…066) | 2026-10-29 | [SIM.md](SIM.md#sim-066-mental-breaks) |
| SND-001-audio-runtime | Audio runtime | 2026-10-21 | [SND.md](SND.md#snd-001-audio-runtime) |
| SND-002-audio-asset-pipeline | Audio asset pipeline | 2026-10-21 | [SND.md](SND.md#snd-002-audio-asset-pipeline) |
| SND-003-animal-voices | Animal voices (281 ElevenLabs clips) | 2026-10-21 | [SND.md](SND.md#snd-003-animal-voices) |
| SND-004-moss-backend | MOSS-SoundEffect backend for creature voices | 2026-11-01 | [SND.md](SND.md#snd-004-moss-backend) |
| SND-005-moss-replaces-elevenlabs | MOSS replaces every ElevenLabs sound | 2026-11-06 | [SND.md](SND.md#snd-005-moss-replaces-elevenlabs) |
| STK-AG-architect-gap | Architect gap analysis and placeholder cards | 2026-11-01 | [STORAGE.md](STORAGE.md#stk-ag-architect-gap) |
| STK-AL-2-drops | What drops on the map (allow fixes) | 2026-11-02 | [STORAGE.md](STORAGE.md#stk-al-2-drops) |
| STK-AL-allow-system | Allow system gap analysis and fixes | 2026-11-01 | [STORAGE.md](STORAGE.md#stk-al-allow-system) |
| STK-AR-bench-art | Production bench art at vanilla size, planned and dump icons | 2026-11-01 | [STORAGE.md](STORAGE.md#stk-ar-bench-art) |
| STK-BN-production-benches-3x1 | Production benches at the wiki footprints | 2026-11-01 | [STORAGE.md](STORAGE.md#stk-bn-production-benches-3x1) |
| STK-SH-colonist-shadows | Only colonists outdoors cast a shadow | 2026-11-01 | [STORAGE.md](STORAGE.md#stk-sh-colonist-shadows) |
| STK-ST-stacks-and-dumping | Stacks of 1000 and the dumping stockpile | 2026-11-01 | [STORAGE.md](STORAGE.md#stk-st-stacks-and-dumping) |
| STO-1-storage-index | Storage index (behaviour-neutral) | 2026-10-30 | [STORAGE.md](STORAGE.md#sto-1-storage-index) |
| STO-2-stockpile-simulation | Stockpile simulation | 2026-10-30 | [STORAGE.md](STORAGE.md#sto-2-stockpile-simulation) |
| STO-3-stockpile-ui | Stockpile UI | 2026-10-30 | [STORAGE.md](STORAGE.md#sto-3-stockpile-ui) |
| SURV-001-harvest-starvation | Survival gate: harvest and starvation | 2026-09-16 | [BASE.md](BASE.md#surv-001-harvest-starvation) |
| SYS-001-save-load | Save and load | 2026-10-06 | [OPS.md](OPS.md#sys-001-save-load) |
| TEMP-0-gap-analysis-and-contract | Temperature gap analysis and contract | 2026-10-31 | [BASE.md](BASE.md#temp-0-gap-analysis-and-contract) |
| TEMP-1-room-thermodynamics | Room thermodynamics | 2026-10-31 | [BASE.md](BASE.md#temp-1-room-thermodynamics) |
| TEMP-2-climate-and-events | Outdoor climate and events | 2026-10-31 | [BASE.md](BASE.md#temp-2-climate-and-events) |
| TEMP-3-exposure | Pawn and animal exposure | 2026-10-31 | [BASE.md](BASE.md#temp-3-exposure) |
| TEMP-4-layered-clothing | Layered clothing | 2026-10-31 | [BASE.md](BASE.md#temp-4-layered-clothing) |
| TEMP-5-ui | Temperature UI | 2026-11-01 | [BASE.md](BASE.md#temp-5-ui) |
| TEMP-6-close-out-simulation | Close-out simulation | 2026-11-01 | [BASE.md](BASE.md#temp-6-close-out-simulation) |
| TEMP-art-climate-and-clothing | Climate buildings and layered clothing art | 2026-10-31 | [BASE.md](BASE.md#temp-art-climate-and-clothing) |
| TIME-001-day-periods-lighting | Day periods and outdoor light | 2026-09-08 | [UI.md](UI.md#time-001-day-periods-lighting) |
| TIME-002-soft-daylight-overlay | Softer daylight overlay | 2026-09-08 | [UI.md](UI.md#time-002-soft-daylight-overlay) |
| TIME-003-one-x-tick-calendar | One-x tick calendar | 2026-09-08 | [UI.md](UI.md#time-003-one-x-tick-calendar) |
| TIME-004-full-dual-time-surface | Full dual-time surface | 2026-09-08 | [UI.md](UI.md#time-004-full-dual-time-surface) |
| TIME-005-clear-day-overlay | Clear day overlay | 2026-09-16 | [UI.md](UI.md#time-005-clear-day-overlay) |
| TOKEN-001-usage-guards | Usage guards and scorecard | 2026-11-02 | [OPS.md](OPS.md#token-001-usage-guards) |
| UI-000-agent-pipeline-integration | Wire the screens spec into the agent pipeline | 2026-09-08 | [UI.md](UI.md#ui-000-agent-pipeline-integration) |
| UI-001-first-session-screens | First-session screens | 2026-09-13 | [UI.md](UI.md#ui-001-first-session-screens) |
| UI-002-warm-ui-palette | Warm UI palette | 2026-09-13 | [UI.md](UI.md#ui-002-warm-ui-palette) |
| UI-003-menu-text-layout | Menu text layout | 2026-09-15 | [UI.md](UI.md#ui-003-menu-text-layout) |
| UI-004-colony-hud-shells | Colony HUD shells | 2026-09-15 | [UI.md](UI.md#ui-004-colony-hud-shells) |
| UI-005-soil-hover-panel | Soil hover panel | 2026-10-06 | [UI.md](UI.md#ui-005-soil-hover-panel) |
| UI-006-camera-bounds | Camera bounds | 2026-10-06 | [UI.md](UI.md#ui-006-camera-bounds) |
| UI-007-order-queue | Per-colonist order queue | 2026-10-06 | [UI.md](UI.md#ui-007-order-queue) |
| UI-008-work-priorities-and-schedules | Work priorities and schedules | 2026-10-06 | [UI.md](UI.md#ui-008-work-priorities-and-schedules) |
| UI-009-threat-pause-and-damage-numbers | Pause on threat, damage numbers | 2026-10-06 | [UI.md](UI.md#ui-009-threat-pause-and-damage-numbers) |
| UI-010-roof-overlay-and-tools | Roof overlay and roof tools | 2026-10-20 | [UI.md](UI.md#ui-010-roof-overlay-and-tools) |
| UI-011-inspectors-move-grass-trees | Inspectors, paused actions, move/copy, grass, tree zones | 2026-10-20 | [UI.md](UI.md#ui-011-inspectors-move-grass-trees) |
| UI-012-options-and-settings | Options and settings | 2026-10-21 | [UI.md](UI.md#ui-012-options-and-settings) |
| UI-013-paths-draft-logs | Paths, draft orders, logs | 2026-10-21 | [UI.md](UI.md#ui-013-paths-draft-logs) |
| UI-014-biome-picker-wildlife-tab | Biome picker, Wildlife tab, animal inspector | 2026-10-22 | [UI.md](UI.md#ui-014-biome-picker-wildlife-tab) |
| UI-015-frame-inspector-carcass-art | Frame inspector, carcass art, materials alert | 2026-10-22 | [UI.md](UI.md#ui-015-frame-inspector-carcass-art) |
| UI-016-input-system | Input system | 2026-10-22 | [UI.md](UI.md#ui-016-input-system) |
| UI-017-link-grid-smooth-tool-stone-readouts | Link grid, Smooth tool, stone readouts | 2026-10-22 | [UI.md](UI.md#ui-017-link-grid-smooth-tool-stone-readouts) |
| UI-020-new-game-flow | New-game flow, widget kit, real font, in-colony additions (NG-1) | 2026-10-22 | [UI.md](UI.md#ui-020-new-game-flow) |
| UI-021-door-over-wall | Door over wall (PLAY-010) | 2026-10-23 | [UI.md](UI.md#ui-021-door-over-wall) |
| UI-022-orientation-and-material-swap | Door orientation and material swap (PLAY-011) | 2026-10-24 | [UI.md](UI.md#ui-022-orientation-and-material-swap) |
| UI-022-social-tab-history | Social tab, health, assign, history UI (ENG-003 M4) | 2026-10-24 | [UI.md](UI.md#ui-022-social-tab-history) |
| UI-023-plan-tool | Plan tool (PLAY-012) | 2026-10-24 | [UI.md](UI.md#ui-023-plan-tool) |
| UI-023-social-finish | Social finish: animal layout, dev hooks, scenario | 2026-10-24 | [UI.md](UI.md#ui-023-social-finish) |
| UI-024-ui-scaling | UI scaling (PLAY-012) | 2026-10-24 | [UI.md](UI.md#ui-024-ui-scaling) |
| UI-025-areas-pens-fire-ui | Areas, pens, fire, snow UI (PLAY-015 UI) | 2026-10-24 | [UI.md](UI.md#ui-025-areas-pens-fire-ui) |
| UI-026-play017-scale-fixes | Scale fixes, zone labels, hauling, chase UI (PLAY-017) | 2026-10-25 | [UI.md](UI.md#ui-026-play017-scale-fixes) |
| UI-027-play018-hud-relayout | HUD relayout and learning helper (PLAY-018) | 2026-10-25 | [UI.md](UI.md#ui-027-play018-hud-relayout) |
| UI-028-play020-zones-beds-lamps | Zones, beds, lamps, bed rest (PLAY-020) | 2026-10-25 | [UI.md](UI.md#ui-028-play020-zones-beds-lamps) |
| UIS-4 | Options, letters and window sizes | 2026-11-07 | [UI.md](UI.md#uis-4) |
| UIS-gap-analysis | UI size and scale gap analysis | 2026-11-06 | [UI.md](UI.md#uis-gap-analysis) |
| UIS-implementation | Gameplay UI sizes and scaling to the reference | 2026-11-07 | [UI.md](UI.md#uis-implementation) |
| UIS-label-fit | Command labels stay inside their cells | 2026-11-07 | [UI.md](UI.md#uis-label-fit) |
| UPDATE-001-update-notice | Running version and update notice | 2026-11-03 | [OPS.md](OPS.md#update-001-update-notice) |
| USAGE-001-usage-data | Usage data client | 2026-11-03 | [OPS.md](OPS.md#usage-001-usage-data) |
| VIEW-001-camera-restored-on-load | Camera restored when a game loads | 2026-10-30 | [ART.md](ART.md#view-001-camera-restored-on-load) |
| WG-0-gap-analysis-and-contract | World generation gap analysis and contract | 2026-11-02 | [WORLD.md](WORLD.md#wg-0-gap-analysis-and-contract) |
| WG-1-planet-v1-and-relief | Planet v1 domain and the globe relief | 2026-11-02 | [WORLD.md](WORLD.md#wg-1-planet-v1-and-relief) |
| WG-2-globe-lines-and-landing-ui | Roads, rivers and labels on the globe; landing UI; units | 2026-11-02 | [WORLD.md](WORLD.md#wg-2-globe-lines-and-landing-ui) |
| WRK-001-work-order | Work order and harvesting | 2026-11-04 | [BASE.md](BASE.md#wrk-001-work-order) |
| ZONE-001-per-tile-zones | Per-tile zones | 2026-10-21 | [STORAGE.md](STORAGE.md#zone-001-per-tile-zones) |
