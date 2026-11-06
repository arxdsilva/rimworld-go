---
name: rimworld-go-ui-screens
description: Implement or review rimworld-go player-facing screens, windows, overlays, HUD layout, navigation, inspectors, and management tables using the canonical main-screens spec. Use whenever a named screen or its shared navigation/chrome is in scope.
---

# Main UI Screens

Canonical spec: `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`.

Ordered backlog: `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`.

Visual chrome/icons: `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` via `rimworld-go-art-assets`.

Do not paste the screens spec into prompts. Read it by section. Do not load it for headless simulation-only work with no screen.

## When this skill applies

Use for:

- boot, menus, new-game flow, pause/save/load/options;
- main colony HUD, camera/selection/time chrome, colonist bar;
- inspectors, pawn tabs, architect, work/schedule/assign, production, research;
- animals, wildlife, prisoners, trade, factions, history, quests, alerts;
- world map, caravans, transport pods, endgame, victory;
- tutorial/help, plugin manager, developer/debug/profiler;
- large-colony management, global search/command palette;
- shared navigation, window/modal/overlay rules.

Do not put screen widgets, focus, or layout into `internal/domain`. Screens consume read models and emit typed commands.

## Required context

Always:

- root `AGENTS.md`;
- this skill;
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` §0 (architecture), §58 (screen vs window vs overlay), §59 (definition of done).

Then only the screen family for the task:

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
| Implementation waves / navigation / inventory | §56–§57, §60 |

Also load:

- matching item(s) in `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`;
- matching gameplay item(s) in `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`;
- named ART_DIRECTION sections when the screen includes original chrome/icons;
- presentation package `AGENTS.md` and existing read-model / command interfaces.

## Architecture

```text
gameplay feature
-> read model / RenderSnapshot
-> screen (adapter)
-> typed CommandEnvelope
-> application / domain
```

- No screen mutates authoritative simulation state.
- Search/filter/sort against presentation data, not a per-frame world scan.
- Screen animations never affect simulation.
- Plugin content appears through registries, not hardcoded switches.
- Tables that can grow to hundreds of rows are virtualized.
- Disabled actions explain why. Escape closes the topmost modal/panel first.

## Workflow

1. Identify the named screen (or shared primitive) and whether it is a full screen, window/panel, overlay, or modal (§58).
2. Confirm Wave UI-0 framework (primitives, focus, window/modal, tooltips, virtual lists, snapshot boundary, command gateway) exists; if not, build that before a later-wave screen.
3. Restate Purpose / Entry / Information / Actions / Commands / States from the named section as acceptance criteria.
4. Implement against read models and typed commands only.
5. Cover empty, error, disabled, stale EntityID, and large-colony states.
6. If chrome/icons are new, follow `rimworld-go-art-assets`.
7. Update matching checkboxes in `MAIN_UI_SCREENS_SPECIFICATION.md` (§56 waves and §0/§59 items), plus the UI/UX and vanilla checklists when those items land.
8. Write a concise handoff including screen/UI impact.

## Task packet extras

```text
screen:
form (screen / window / overlay / modal):
entry / navigation:
commands emitted:
read models consumed:
scale requirement:
```

## Completion gate

A screen is done only when §59 is met, including:

- underlying gameplay exists;
- read models + typed commands;
- navigation, Escape/back, focus, empty/error/disabled states;
- virtualized large lists;
- no per-frame world scan;
- tests where practical;
- visual acceptance screenshot/scenario;
- matching checkboxes updated;
- handoff written.
