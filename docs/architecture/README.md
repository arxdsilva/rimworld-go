# Architecture diagrams

Diagrams for the game architecture gap analysis (GAR, `.agents/gaps/GAME_ARCHITECTURE_GAP_ANALYSIS.md`). Each one is an Archify candidate (`<name>.json`) plus the page built from it (`<name>.html`). Archify also writes a `<name>.delivery.json` provenance record next to each page; it holds this machine's absolute paths, so it is not committed (`.gitignore`). "Today" means the code at commit `c27bd1512`, with sources pinned to that commit. "Target" means the design in the named ADR (`docs/adr/`).

| Diagram | What it shows | State |
|---|---|---|
| [system-architecture](system-architecture.html) | The layers (cmd, compose, inbound adapters, application, domain, outbound adapters, platform, pkg), with each inward import labelled by its consumer-owned port | today |
| [cmd-wiring](cmd-wiring.html) | Each `cmd/*/main.go` and the cli package or composition root it calls | today |
| [frame-sequence](frame-sequence.html) | One frame: Ebitengine calls Update (advanceColony under the 5 ms budget, then Game.Step × n, then post-step work), then Draw (renderer layers, HUD), then present at vsync | today |
| [frame-budget](frame-budget.html) | The 16.67 ms budget segments against the measured probe averages at zoom 0.8 and 0.4 (illustrative) | today and target |
| [tick-phases](tick-phases.html) | The 15 tick phases in order, how often each runs, and the order constraints | today |
| [periodic-calendar](periodic-calendar.html) | The global 250-tick, hourly and daily gates piling up on the day tick, against the staggered target (synthetic clock) | today and target (ADR 0001) |
| [scheduler](scheduler.html) | One tick under the systems scheduler: systems in phase order, a parallel stage, event drains | target (ADR 0001, 0002, 0003) |
| [event-flow](event-flow.html) | Commands, timers and systems emit into the event queue, then dispatch, handlers, stores, query ports, the event log and replay | target (ADR 0003) |
| [parallel-stage](parallel-stage.html) | A parallel stage: Plan on the owner, Compute on the pool lanes, Apply in entity-ID order | target (ADR 0002) |
| [read-model-flow](read-model-flow.html) | Stores to World() and Snapshot(), memoised on [tick, viewGen], to the renderer layers and the HUD | today |
| [render-pipeline](render-pipeline.html) | The world layers in draw order, each with its draw style, then the composite and the HUD | today |
| [render-cmdlist](render-cmdlist.html) | Layers emit keyed commands into an arena, then a sorted check or radix sort, a merge, and Submit | target (ADR 0004) |
| [autosave-lifecycle](autosave-lifecycle.html) | The synchronous 06:00 autosave on Update, and the async save job that replaces it | today and target (GAR-190) |
| [target-architecture](target-architecture.html) | Diagram 1 after Part E: game/sched, domain/sim/event, platform/pool, render/cmdlist and game/savejob added | target (ADR 0001 to 0004) |
| [architecture-delta](architecture-delta.html) | Archify's compare of system-architecture against target-architecture | today to target |

## Re-rendering

Edit the candidate JSON, then run this from the repository root (`<type>` is the candidate's `diagram_type`):

```sh
ARCHIFY_UPDATE_CHECK_DISABLED=1 node ~/.claude/skills/archify/bin/archify.mjs \
  finalize <type> docs/architecture/<name>.json docs/architecture/<name>.html \
  --quality showcase --repo-root . --out-dir "$TMPDIR/archify/<name>" --json
```

`--repo-root` checks the cited line ranges against the pinned commit. It is needed only for candidates that have `meta.repository`. If you move the pin to a new commit, recheck every cited range. `--out-dir` keeps the finalize and browser-check receipts out of the repository. To rebuild the delta:

```sh
ARCHIFY_UPDATE_CHECK_DISABLED=1 node ~/.claude/skills/archify/bin/archify.mjs compare architecture \
  docs/architecture/system-architecture.json docs/architecture/target-architecture.json \
  docs/architecture/architecture-delta.html --receipt docs/architecture/architecture-delta.receipt.json \
  --quality showcase --repo-root .
```

Compare requires its receipt to sit beside the HTML, so `architecture-delta.receipt.json` is kept in this directory.

Finalize opens a headless browser, so don't run it while a frame-rate probe (`make fps-matrix`) is measuring.
