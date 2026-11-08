# Game architecture gap analysis (GAR)

Status: in progress (2026-11-08). The research, the baseline and the rows land in Part D of the GAR plan; this file is their home.

## Scope

- **What:** game-architecture practices (scheduling, parallelism, events, render batching, frame pacing, runtime tuning) compared with ours, and the move to the architecture the user chose.
- **Target:** at least 60 fps, always, on an Apple M2 Max (12 CPU and 38 GPU cores, 64 GB) driving a 5120×2880 backing at 60 Hz, with VSync on. The workload is today's play: the grown 500×500 map with about 20 colonists, every zoom, 1x/3x/6x, raids included.
- **Sources:** public books, talks, papers, engine docs and source, for ideas only, each with its URL (CLEAN_ROOM.md, GAR row). Nothing about RimWorld's internals.

## Decision record (2026-11-08)

| axis | choice |
| --- | --- |
| Sim core | Systems scheduler over the existing `component.Store[T]` stores |
| Threading | Parallel pure stages: workers read a frozen view and return results that the single scheduler goroutine applies in entity-ID order |
| Messaging | An event queue for every cross-system effect; systems read each other only through query ports |
| Rendering | A render-command list: layers emit sort-keyed commands, merged into the fewest draw calls |

Also decided:
- Every `cmd/<tool>` is one wiring-only `main.go` (SOLID_GO.md §8a).
- VSync stays on in every measurement.
- Determinism holds: same seed and commands, same state hash.
