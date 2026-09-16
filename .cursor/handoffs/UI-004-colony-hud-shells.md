# Task Handoff

Task ID: UI-004
Status: complete

## Summary

Colony play now has original HUD chrome instead of debug overlay text: a resource/time strip, colonist bar, bottom management tabs, a selection inspector window, and a Work overlay. Snapshot-backed where `PawnView` exists; resources, work cells, and most tabs are stubs. Layout follows spec §9 / §57 names (Architect, Work, Schedule, Assign, Animals, Research, World, History, Factions, Menu), not proprietary RimWorld chrome or labels.

## Files Changed

- `internal/adapters/inbound/ebiten/hud.go`
- `internal/adapters/inbound/ebiten/inspector.go`
- `internal/adapters/inbound/ebiten/work.go`
- `internal/adapters/inbound/ebiten/app.go`
- `internal/adapters/inbound/ebiten/renderer.go`
- `internal/adapters/inbound/ebiten/hud_test.go`
- `internal/adapters/inbound/ebiten/AGENTS.md`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

## Public Contracts Changed

`Renderer` gained presentation-only `Select`, `ClearSelect`, `HasSelect`, and `Selected`. App drives bar/inspector selection through that API. No simulation command catalog.

## Invariants / Decisions

- HUD, inspector, and Work consume pointer hits before map selection.
- Escape on colony: close Work, then clear inspector selection, then pause.
- Work cells are labels only; they never call `Step` or mutate sim.
- Disabled tabs stay visible with reason `not implemented`. Menu opens the existing pause overlay.
- Inspector is a window (Character tab live; Needs/Health/Social/Gear disabled). Work is an overlay on the colony screen, not a full-screen route.
- Resource counts are placeholders. Colonist names are `Colonist N`. Portraits use the vendor idle frame when the pack is loaded.

## Verification

Tests: `go test ./internal/adapters/inbound/ebiten/`
Benchmarks: none.

Headless coverage: bar click selects; Work tab toggles overlay; disabled tabs do not navigate; Escape closes Work then inspector then pause; Work cells are not buttons and do not extra-`Step`.

In-game window was not visually QA'd in this session. Run `go run ./cmd/game`, New Game → Start, then bar / inspector / Work / Escape.

## Determinism Impact

None. HUD is presentation-only. Speed keys still call `Step(n)` at the existing rates.

## Save / Mod Impact

None.

## Art / Visual Impact

No new sprites. Chrome reuses `assets/palettes/ui.yaml` umber panels. Portraits reuse the vendor colonist idle frame. Spec screenshots were information architecture only.

## Screen / UI Impact

Main Colony (§9) now has HUD regions. Generic Inspector (§10) is a pawn Character-tab shell. Work (§19) is a non-virtualized overlay. Wave UI-1 inspector/bar/tabs and Wave UI-2 Work are `[-]`. UI/UX §3.1 / §4.1 / §4.2 / §7.1 matching items are `[-]` shells.

## Known Limitations

- No economy, so resource numbers are stubs.
- No work priorities, schedule grid, architect, or selection command bar.
- Work table is not virtualized (typical snapshot is a handful of pawns).
- No double-click camera center, status icons, or pawn names from content.
- §59 DoD is not met.

## Follow-ups

- Bind Work cells to real priorities once the work system exists.
- Inspector Needs/Health/Social/Gear tabs and contextual commands.
- Resource strip from an economy read model.
- Virtualize the Work table before large colonies.
