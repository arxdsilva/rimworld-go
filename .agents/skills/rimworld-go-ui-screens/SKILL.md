---
name: rimworld-go-ui-screens
description: Build or review rimworld-go screens, windows, overlays, HUD, navigation and management tables. Use when a named screen or shared chrome is in scope.
---

# Main UI screens

Spec: `.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` (MAIN_UI). Read by section; skip for headless work with no screen. Covers boot and menus, colony HUD, inspectors and pawn tabs, architect, work/schedule/assign, production, research, animals, prisoners, trade, factions, history, quests, alerts, world map, caravans, pods, endgame, pause/save/load/options, tutorial, plugins, developer tools, large-colony management, search, shared navigation and window/modal/overlay rules. Widgets, focus and layout never enter `internal/domain`.

## Load

- Root `AGENTS.md`; MAIN_UI §0 Architecture (read model -> screen -> typed command; the canonical flow), §58 Screen vs window vs overlay, §59 Definition of done.
- Then only the screen family:

| Family | Sections |
|---|---|
| Boot / menus / new game | §1–§8 |
| Main colony HUD | §9 |
| Inspectors / pawn screens | §10–§16 |
| Architect / zones | §17–§18 |
| Work / schedule / assign / policy | §19–§22 |
| Production / research | §23–§24 |
| Animals / wildlife / prisoners | §25–§27 |
| Trade / factions / history / quests / alerts | §28–§32 |
| World map / caravan / pods / endgame | §33–§38 |
| Pause / save / load / options | §39–§42 |
| Tutorial / help | §43–§44 |
| Plugins | §45–§48 |
| Developer / profiler | §49–§53 |
| Large colony / search | §54–§55 |
| Waves / navigation / inventory | §56–§57, §60 |

- The matching rows of `.agents/backlog/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` and the vanilla checklist; ART sections (via `rimworld-go-art-assets`) when chrome or icons are new; the presentation package `AGENTS.md` and its read-model and command interfaces.

## Rules local to screens

- Search, filter and sort against presentation data, never a per-frame world scan (MASTER §55, §78).
- Screen animation never affects simulation. Plugin content appears through registries, not switches.
- Tables that can reach hundreds of rows are virtualized.
- Disabled actions explain why. Escape closes the topmost modal or panel first.

## Steps

1. Name the screen or shared primitive and its form: screen, window/panel, overlay or modal (§58).
2. Confirm the Wave UI-0 framework exists (primitives, focus, window/modal, tooltips, virtual lists, snapshot boundary, command gateway); build it before any later-wave screen.
3. Restate Purpose / Entry / Information / Actions / Commands / States from the named section as acceptance criteria.
4. Implement against read models and typed commands only.
5. Cover empty, error, disabled, stale EntityID and large-colony states.
6. Follow `rimworld-go-art-assets` for new chrome or icons.
7. Tick the MAIN_UI wave (§56) and DoD (§0/§59) items, and the UI/UX and vanilla rows (root `AGENTS.md` checkbox rule).
8. Append a section to the family ledger (`.agents/templates/HANDOFF.md`) with the screen impact.

## Task packet extras

On top of `.agents/templates/TASK_PACKET.md`: screen, form (screen / window / overlay / modal), entry / navigation, commands emitted, read models consumed, scale requirement.

## Completion gate

MAIN_UI §59 is met: the underlying gameplay exists; read models and typed commands; navigation, Escape/back, focus, and empty/error/disabled states; virtualized large lists; no per-frame world scan; tests where practical; a visual acceptance screenshot or scenario; checkboxes updated; ledger section appended.
