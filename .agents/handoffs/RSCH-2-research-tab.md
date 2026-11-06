# RSCH-2: the Research tab as RimWorld's research screen

Agent U, wave 1 (2026-11-01). Skill rimworld-go-ui-screens, MAIN_UI §24. Not committed (the lead integrates).

## What changed
- **`windows/research.go`** (rewritten), **`research_tree.go`**, **`research_panel.go`**, **`research_layout.go`** (new):
  - **Tech tree.** One column band per tech level. Inside a band, a project sits one column right of its deepest same-level prerequisite.
  - **Rows.** Ordered by barycentre sweeps; the order with the fewest crossings is kept. Rows are then placed to line up with prerequisites, so chains run straight.
  - **Drawing.** Prerequisite lines come from `ResearchView.Prereqs` (hidden ones are not drawn); the window reads no domain project table. Done boxes are green, available ones dark, unavailable ones grey. The current project is outlined and fills with progress. A second text line states the state in words.
  - **Pan and search.** Drag on empty space, middle-drag, wheel or Shift+wheel pans the tree, clamped to the content. Search dims non-matches, shows "N found", and Enter jumps to the first match. Selecting a box highlights its whole prerequisite chain.
  - **Info panel.** Name, tech level, description, "X / Y" (Progress×CostNow/Cost out of CostNow; the percent is on the base Cost) and the ETA. Prerequisites are links that select and centre the project. Hidden prerequisites appear as text. Bench and facility lines turn red by `game/research.MissingBench` / `MissingHiTech` / `MissingAnalyzer`, with the reason below. Then unlocks, Start research (disabled with "Needs X first", or Finished / Researching), and "Debug: finish now" in developer mode (not shown in commitment colonies). The panel body scrolls.
  - **Opening.** The tab opens on the current project, else on the first available one.
  - **Caches.** The layout is cached on the research list's identity and project set; node texts and panel lines are rebuilt only when their fields change. The tree's own per-frame work allocates nothing; the gfx primitives allocate internally, as they do everywhere in the HUD.
- **`windows/deps.go`**: one field, `research *researchState`.
- **`hud/alerts.go`**: the "Need research project" alert (key `research`) from `WorldView.ResearchAlert`. Its explanation tells the player what to do, and a click opens the tab (a `ports.Dispatcher` assertion).
- **`hud/tabs.go`**: unchanged; its fill is Progress/Cost, which is right at base cost.
- **`inspector/research_lines.go`** (new) plus one `add` line in `cmds_build.go`:
  - research benches show the project under way and "Research speed: N%";
  - the hi-tech bench says whether a multi-analyzer is linked;
  - the analyzer counts the benches that link to it.
- **`learn/lessons.go`**: the research lesson text now speaks of the tree.

## Ids and inputs (for J)
- No new input actions.
- Widget ids:
  - `research.pick.<id>` (existing prefix; now selects the project);
  - `research.tree`, `research.search`, `research.info`, `research.start`, `research.devfinish`, `research.prereq.<id>`;
  - `alert.research`.
- New ids are local constants in `windows/research_tree.go`.

## Tests
- windows: layout columns, bands and crossings; table order never beats the layout; chain, links, start and finish; search; pan; caches and allocations.
- hud: the research alert.
- inspector: research lines.
- root:
  - `research_tree_test.go` replaces `research_scroll_test.go`: every project is reachable and a box can be started at 1280x720, at 75 % / 150 % / 200 % UI scale and at 1113x626; the window draws only inside itself at every pan; drag; opening on the current project; alert; dev finish.
  - `research_ui_test.go`, `journey_tabs_test.go` (plus `TestJourneyTabsResearchDevFinish`) and `windows_test.go` are updated to select-then-start.

## Player-visible
Everything above. The completion letter needs nothing from the UI: the existing letter window shows the title and text that RSCH-1 posts.
