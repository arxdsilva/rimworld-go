# Task Handoff

Task ID: UI-023 (PLAY-012)
Status: complete

## Summary
Plan / Remove plan (Architect › Orders, filled rectangle or oval) paint purely visual layout marks. Nobody works on them, they block nothing and they change no room, path or zone. A mark disappears when any building (wall, door, fence, furniture…) is finished or moved onto its tile; floors and in-place material swaps leave it. The overlay (soft blue fill, dashed edges) is on by default; the HUD "Plans" button hides it and holding a plan tool shows it. Hovering a marked tile reads "Planned".

## Files
`game/plan.go` (layer, `paintPlan`, clear hooks in `completeFrame`/`completeMove`, sparse save `Plan []int32`, view `WorldView.Plan`/`PlanVer`), command `DesignatePlan`; renderer `plan_draw.go`, `archtools.go` tools, `hud.go` button/hover line, icons `ui_icon/plan`, `plan_clear` (overlay.py), palette `ui.plan_fill`/`ui.plan_edge`.

## Tests
`plan_test.go`, `plan_ui_test.go`, `plan_draw_test.go`. Goldens unchanged (the save field is nil without marks).
