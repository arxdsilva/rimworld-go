# ADR 0003: an event queue for every cross-system effect

Status: accepted (2026-11-08). Gap rows: GAR-050 to GAR-079.

## Context

Systems today reach each other through host interfaces that `Game` implements: a sub-package calls back into `Game`, and `Game` mutates another system's state on the spot, depth-first. Host interfaces keep growing, the order of side effects is implicit in call nesting, and nothing records what happened in a tick except the state hash. `cmd/replay` is a stub.

## Options

| | Direct host calls (today) | **An event queue everywhere (chosen)** |
|---|---|---|
| pros | Explicit at the call site; nothing to learn | Decoupled; one total order that a log can record; features subscribe without touching emitters; replay and diagnosis |
| cons | Host interfaces grow; implicit order; no log | Harder to follow a chain without tools; drain order becomes a determinism rule; the risk of a hidden global bus |

## Decision

1. **Events are facts in the past tense** (`PawnDowned`, `ItemHauled`). A request with exactly one owner stays a typed command (external input), or becomes a named intent event whose single handler the dispatch table records.
2. **Typed columns plus one spine, no boxing.**
   - Each kind has a plain value payload: entity IDs with generation, cells, amounts, def IDs. Payloads hold no pointers into state.
   - Each kind is stored in its own reused `Column[T]`.
   - One spine `[]Ref{Kind, Idx, Seq, Cause}` records global emit order.
   - Dispatch walks the spine with an exhaustive switch on `Kind`.
   - Adding a kind without a handler entry, a codec or a log format fails `make arch`.
   - The package is `internal/domain/sim/event`.
3. **One queue per sim, owned by the tick driver**, never a package variable. A tick runs commands, then timers, then systems in schedule order (ADR 0001), then the end-of-tick checks. Systems emit; only the driver drains. To begin with there is one drain after each emitting system, which keeps today's order; drains get coarser only on purpose, with a re-pinned golden.
4. **Re-entrant events go FIFO, with generations and caps.** An event emitted while draining goes to the tail of the same drain, with generation `parent + 1`. Caps on generation depth and on events per tick turn a feedback loop into a test failure. Handlers never call handlers. Each feedback pair (A → B → A) gets a no-flap test.
5. **Order is a contract.**
   - The total order is `Seq`, the emit order, reset every tick.
   - Handlers per kind form a static slice built at the composition root, with no `init()` and no map.
   - A golden file of the dispatch table makes any reordering a reviewed change.
6. **The queue is empty at every tick's end.** The driver asserts it, and save asserts it, so the queue is in neither the state hash nor the save. Anything that crosses a tick is domain state or a timer.
7. **A saved timer heap.**
   - Entries `{Due tick, Seq, Kind, payload, Cause}` are ordered by `(Due, Seq)`, with `Seq` from a counter stored in the save.
   - Cancellation uses `{slot, generation}` handles.
   - The heap is a small typed generic one, not `container/heap`, which boxes its elements.
   - Due timers emit events in the timers phase.
   - Introducing the heap is a save migration from today's ad-hoc countdown fields.
8. **Query ports are read-only.**
   - Ports have at most 8 methods. They return values or ID iterators, never pointers or slices into storage, and never take an emitter.
   - A handler receives only (event, its query ports, an emitter, its own system's state).
   - `TestQueryPortsDoNotMutate` combines a static check with a purity test: hash the state, call every query, hash again.
   - Its allowlist of mutators only shrinks.
9. **Payloads can go stale.** Handlers re-validate targets through a port (alive? same generation?). A missing target is a logged no-op.
10. **The event log is derived, not authoritative.** Replay input stays the save, the seed and the command stream. The log records per tick:
    - the state hash after the tick;
    - the commands;
    - the events in `Seq` order, each with generation, cause and a fixed-width payload.

    Its header carries the build, defs and schema hashes. `cmd/replay` re-simulates and reports the first differing tick and event (GAR E10). Logging sits behind an injected sink that is off by default and costs nothing when off.

    UI toasts and audio read a per-tick outbox copied after the tick, never the queue.
11. **Migration by parallel change, one seam at a time.**
    - Each seam goes through three steps: add the kind, emit and handler; move one call site; delete the direct call.
    - The oracle is an identical per-tick state hash on every golden scenario plus a seeded command soak. The drain sits right after the emitter.
    - A seam where the old call was nested depth-first inside another handler can legitimately change results. Such a seam lands in its own commit, with the reason. Leaf effects (notifications, stats, sounds) move first, core chains later.
12. **No allocations.** Columns, spine and heap are preallocated and reused, with no closures, `any` or `sync.Pool`. `AllocsPerRun` must be 0 for an emit and drain at steady state.
13. **Debugging is built in.** A log viewer filters by kind or entity and walks the cause tree. The read model carries counters per kind and the deepest generation.

## Consequences

- Host interfaces shrink to query ports, and the `Game` methods behind them move into the systems that own the state (GAR E11).
- One new save section, the timer heap. The queue itself never reaches a save.
- The "UI and rendering never mutate" rule extends: handlers mutate only their own system's state.

## Sources

Research fragment R-events, with every URL. Game Programming Patterns (Event Queue, Observer, Command); Bevy events, messages and observers; deterministic lockstep ("1500 Archers", Gaffer); StarCraft II tracker events; Factorio heavy mode; Riot's determinism write-ups; Fowler on event sourcing, CQRS and parallel change; Cox on Go interface allocation.
