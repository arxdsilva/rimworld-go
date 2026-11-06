# SHT-MIN-RSC-0: gap analyses of shooting, mining and rescue

User request (2026-11-05): how shooting, mining and rescue work in RimWorld, what we're missing, and how to use it. Approving the plan is the user's decision extending ENG-003 to SHT-*, MIN-* and RSC-*.

## What changed

- New: `.cursor/SHOOTING_GAP_ANALYSIS.md`, `.cursor/MINING_GAP_ANALYSIS.md`, `.cursor/RESCUE_GAP_ANALYSIS.md`, each with leverage ideas, sourced numbers, rows and proposed phases.
- Corrected: `COMBAT` (Careful row, weather and light, miss radius, interception, XP path, status), `DEFENCE` (lines 37 and 137), `PAWN_INTERACTION` (casket, join, bed rank, Doctor order, +16, margin).
- `DOCUMENT_INDEX.md` has three new rows; `AGENTS.md` extends ENG-003.

## Findings that changed while writing

- Rock HP is published for all five stones (the plan expected sandstone only), so no original tuning is needed there.
- Gun cooldown is not a gap: Aiming Time says it's unaffected, and the only published cooldown factor is Biotech.
- Stone falls to half per tile, not a quarter: our stonecutter gives 10 blocks a chunk, the wiki 20, at 25 % of the chunks.
- Plain-rock mining isn't the worst: a steel tile takes 4.2× longer, and a steel vein holds about 7× what ours does.
- New rows: capturing a downed neutral has no cost (RSC-203); shooting a downed target has no rule (SHT-008); `AimCooldown` slows melee (SHT-103); scanners work under a roof (MIN-304).

## Pages

Doctor, Capture and Medical bed return 404. Traits, Events and Factions rows came through a summary and say so in the docs. Re-read before a phase uses them.

## Open, for when a phase is planned

Lethal thick cave-ins, and whether to adopt all mining numbers at once, have recommendations recorded in the mining doc.

## Checklists and behaviour

No Go changes, no checklist ticks: nothing shipped. Each doc lists its cross-references, and rows no checklist has yet. No HUD representation: docs only. RSC-205 (a doctor repeating an unreachable trip) comes from reading the code and has no test yet.
