# Architecture decision records

Each record states the context, the options with their pros and cons, the decision and its consequences. A record changes only by a new record that supersedes it.

| ADR | decision | status |
|---|---|---|
| [0001](0001-systems-scheduler.md) | A systems scheduler over the component stores: declared reads and writes, deterministic order, staggered periodic work | accepted 2026-11-08 |
| [0002](0002-parallel-pure-stages.md) | Parallel pure stages under one writer: Plan, Compute on a fixed pool, Apply in entity-ID order | accepted 2026-11-08 |
| [0003](0003-event-queue.md) | An event queue for every cross-system effect, query ports for reads, a saved timer heap | accepted 2026-11-08 |
| [0004](0004-render-command-list.md) | A render-command list: sort keys, merging, light in vertex attributes | accepted 2026-11-08 |

The research behind them and the full gap list are in `.agents/gaps/GAME_ARCHITECTURE_GAP_ANALYSIS.md` (family GAR). The diagrams are in [`docs/architecture/`](../architecture/README.md).
