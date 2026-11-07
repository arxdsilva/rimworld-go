# Clean room and licensing

The one home of the clean-room policy and of every user-approved exception to it. Root `AGENTS.md`, `.agents/specs/MASTER_SPEC.md` §86, the art spec and the skills point here.

## Policy

Contributors must not:
- decompile or reference a proprietary implementation;
- copy code;
- extract game data or assets;
- copy dialogue, descriptions or other text;
- duplicate exact balancing databases.

Art: never trace, extract, recolor, vectorize or closely reconstruct proprietary RimWorld sprites, textures, icons, UI art, source files or other copyrighted assets, and never use extracted RimWorld assets as image-to-image inputs or ask for a pixel-perfect recreation. Reproduce broad design principles and gameplay readability, not proprietary artwork.

Creatures: never reproduce a RimWorld-invented creature (name, look, sounds or balance); real animals are fine (`.agents/skills/rimworld-go-creatures/SKILL.md`).

Licensing: choose project and asset licenses deliberately. Plausible code choices: MPL-2.0, GPL-3.0, AGPL-3.0, Apache-2.0 or MIT, depending on the reciprocity wanted; assets can use separate licenses. Get legal advice before commercial distribution. Public final branding should be original: `rimworld-go` is a working engineering codename, and a public title should not imply affiliation with another trademark. Every module in `go.mod` is listed in `THIRD_PARTY_NOTICES.md`.

## Exceptions (explicit user decisions)

Numbers from the published RimWorld wiki are tuning data where the table allows them. Each such constant gets `// wiki: <page>`, or `// original tuning` where the wiki publishes nothing. A new exception is a user decision, recorded here as a new row (or a new family in an existing row) in the same change.

| Scope | ID families | Gap file | Decided | May use | Stays original |
|---|---|---|---|---|---|
| Social and host systems (ENG-003, the base exception) | ENG-003 | `.agents/specs/SOCIAL_SYSTEM_CONTRACT.md` | 2026-10-23 | Published RimWorld wiki numbers | Names and prose |
| Medical system | MED-* | `.agents/gaps/MEDICAL_GAP_ANALYSIS.md` | 2026-10-26 | Wiki numbers | Names and prose |
| Pawn-to-pawn care and mental breaks | SIM-061…066 | `.agents/gaps/PAWN_INTERACTION_GAP_ANALYSIS.md` | 2026-10-29 | Wiki numbers | Names and prose |
| Beds, production and defence | BED-*, PRD-*, DEF-* | `.agents/gaps/BEDS_GAP_ANALYSIS.md`, `.agents/gaps/PRODUCTION_GAP_ANALYSIS.md`, `.agents/gaps/DEFENCE_GAP_ANALYSIS.md` (`.agents/gaps/COMBAT_GAP_ANALYSIS.md` applies the defence extension) | 2026-10-29 | Wiki numbers | Names and prose |
| Food production | FOOD-* | `.agents/gaps/FOOD_GAP_ANALYSIS.md` | 2026-10-30 | Wiki numbers | Names and prose (generic crop names such as rice are fine) |
| Temperature management | TEMP-* | `.agents/gaps/TEMPERATURE_GAP_ANALYSIS.md` | 2026-10-31 | Wiki numbers | Names and prose |
| Research | RSCH-* | `.agents/gaps/RESEARCH_GAP_ANALYSIS.md` | 2026-11-01 | Wiki numbers | Names and prose |
| Storage and the Architect | STK-*, AX-* | `.agents/gaps/ARCHITECT_GAP_ANALYSIS.md`, `.agents/gaps/STORAGE_GAP_ANALYSIS.md` | recorded 2026-11-02 | Wiki numbers | Names and prose |
| The globe and multiple maps | WG-*, MAP-* | `.agents/gaps/WORLDGEN_GAP_ANALYSIS.md`, `.agents/gaps/MULTIMAP_GAP_ANALYSIS.md` | 2026-11-02 | Wiki numbers | Names and prose |
| Power wiring | PWR-* | `.agents/gaps/POWER_GAP_ANALYSIS.md` | 2026-11-03 | Wiki numbers (the 6-tile range, what carries power) | Names and prose |
| Work priorities and harvesting | WRK-*, HRV-* | `.agents/gaps/WORK_PRIORITIES_GAP_ANALYSIS.md`, `.agents/gaps/HARVEST_GAP_ANALYSIS.md` | 2026-11-04 | Wiki numbers | Names and prose |
| Shooting, mining and rescue | SHT-*, MIN-*, RSC-* | `.agents/gaps/SHOOTING_GAP_ANALYSIS.md`, `.agents/gaps/MINING_GAP_ANALYSIS.md`, `.agents/gaps/RESCUE_GAP_ANALYSIS.md` | 2026-11-05 | Wiki numbers | Names and prose |
| Gear tab, outfits and ailments | GEAR-*, OUT-*, AIL-* | `.agents/gaps/GEAR_GAP_ANALYSIS.md`, `.agents/gaps/AILMENTS_GAP_ANALYSIS.md` | 2026-11-06 | Wiki numbers | Names and prose |
| Neat Storage furniture set (mod) | NST-* | `.agents/gaps/NEAT_STORAGE_GAP_ANALYSIS.md` | 2026-11-03 | The mod's published sizes and per-cell capacities | Costs, stuff, beauty, hit points, filters, work and looks |
| RimHUD (mod) | RHUD-* | `.agents/gaps/RIMHUD_GAP_ANALYSIS.md` | 2026-11-03 | Behaviour only: what the mod does | Everything: its repository has no license, so no code, text, names or colours enter this repo; the mod's name appears only in the gap file, never in code or UI |
| Common Sense (mod) | CSE-* | `.agents/gaps/COMMON_SENSE_GAP_ANALYSIS.md` | 2026-11-03 | Behaviour only: what the mod does | Everything: its repository has no license, so no code, text, names or colours enter this repo |
| Gameplay UI sizes and scaling | UIS-* | `.agents/gaps/UI_SCALE_GAP_ANALYSIS.md` | 2026-11-06 | On-screen sizes, proportions, UI scale steps and zoom range measured in the running game (1.6.4871, Core) | Art, icons, colours, fonts, text and code; screenshots never enter the repo |

Notes:
- ENG-003 covers wiki numbers only. A mod's numbers are not the wiki's: Common Sense thresholds recorded in its gap file are our tuning to change, not an ENG-003 use.
- `.agents/gaps/REFUEL_GAP_ANALYSIS.md` adds no numbers; its capacities and burn rates are wiki numbers already admitted under PLAY-021, AX-4 and AX-9.
- Anything not in a row above stays original tuning until the user extends this table.
