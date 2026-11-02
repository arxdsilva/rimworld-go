# WG-0: world generation gap analysis and contract

User request (2026-11-02): "fix the globe map before starting a game", with mountains drawn on the globe. First a gap analysis against vanilla's Create world and Select starting site screens, then fix every gap. Plan: `/Users/arxdsilva/.claude/plans/redraw-the-narrators-they-glimmering-lampson.md` (Part 1).

## What changed
- `.cursor/WORLDGEN_GAP_ANALYSIS.md`: rows WG-01..18 plus three n/a (DLC) rows (pollution, landmass and mountain sliders, landmarks and tile mutators), with evidence lines, the target globe look and a numbers appendix (wiki or original tuning).
- Doc corrections: UI/UX 26.1 (the seed field is free text), 25.1 (the World tab exists), the vanilla rainforest/shrubland note now points at WG-02; `DOCUMENT_INDEX.md` lists the new docs.
- Decisions: ENG-003 is extended to WG-* and MAP-* by the user's approval of the plan. The matching `AGENTS.md` line is NOT written yet (pending the user); do not read AGENTS.md as listing WG, MAP, STK or AX.
- The plan found three stale evidence lines (WG-01, WG-11, MAP-05); the gap docs carry the corrected ones. One premise was wrong: there was no disease constant to replace (see WG-1).

## Gaps
- None open at this phase; implementation is WG-1 and WG-2.
