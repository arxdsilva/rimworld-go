---
name: rimworld-go-ui-screens
description: Use when implementing, changing, or reviewing rimworld-go player-facing screens, windows, overlays, HUD layout, navigation, inspectors, menus, or management tables. Also use for boot/menu/new-game flow and plugin/developer screens. Skip for headless simulation-only work with no screen.
---

# Main UI Screens (Cursor entry)

Canonical playbook: `.agent/skills/rimworld-go-ui-screens/SKILL.md`.

Canonical spec: `.agent/MAIN_UI_SCREENS_SPECIFICATION.md`.

Read the playbook first. Then load only the spec sections it names for this screen. Do not ingest the entire spec by default. Screens consume read models and emit typed commands; they never mutate domain state.
