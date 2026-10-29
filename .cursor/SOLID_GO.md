# rimworld-go — SOLID in Go: Responsibility, Extension and Size Limits

> **Status:** Mandatory coding standard  
> **Owner:** Architecture (lead agent; budget changes only ever lower a line)  
> **Audience:** All human and AI contributors  
> **Enforced by:** `internal/arch/solid_test.go`, budgets in `internal/arch/solid_budget.txt`, run with `make arch`  
> **Companions:** `GO_BEST_PRACTICES.md`, `HEXAGONAL_ARCHITECTURE.md`

---

# 1. Why This Document Exists

The dependency rules in `HEXAGONAL_ARCHITECTURE.md` hold. The size and shape rules did not, and the numbers show it:

- `ebiten.App`: 111 fields, 950 methods over 126 files.
- `ebiten.Renderer`: 74 fields, 398 methods over 64 files.
- `game.Game` (`internal/application/game`): 152 fields, 2105 methods.
- `pawn/body.Body`: 99 methods.
- `ebiten.Simulation`: 32 methods, and the only interface in non-test code with more than 8.
- Two packages over 12,000 non-test lines: `ebiten` (64,178 lines, 212 files) and `game` (57,019 lines, 185 files). The third largest has 3,801.
- 11 `init()` registrations in `ebiten` (`registerScreenUI`) and 5 in domain definition tables.
- Mutable package globals in `ebiten`: `curUIScale`, `tracePass`, `traceVia`, `canvasTrace`, `drawTrace`, `appBackground`, `appWindow`, `animalMarkTrace`.

Reviewers do not hold these limits. The guard does. `make arch` fails the build when a limit is crossed, and the budget file only ever shrinks (§8). This document gives the Go reading of SOLID and the numeric limits. It restates no rule from the other specs.

---

# 2. Go Vocabulary for SOLID

A type is methods on data. It is not a class: there is no hierarchy, no constructor magic and no base type. Responsibility is measured by what state a type owns and which transitions it performs on that state.

Interfaces are satisfied implicitly, so the consumer defines them. The provider never sees the interface and never lists it. An interface is therefore the consumer's list of calls, sized by that consumer.

Embedding composes; it does not inherit. A struct that embeds another gains its methods but substitutes for nothing. Substitutability exists only through interfaces, and only the interface's documented contract binds an implementation.

The package, not the type, is the unit of cohesion and visibility (`HEXAGONAL_ARCHITECTURE.md` §14). A type is split inside its package first; a new package is the last step, not the first.

---

# 3. S — Single Responsibility

**In Go.** One type owns one family of state transitions. One package owns one concept. One file holds one feature of that concept.

**Rule.**

- A type has at most 40 methods.
- A type with behaviour has at most 25 fields. A pure data read model (DTO) is exempt by construction.
- A package has at most 12,000 non-test lines and 60 files.
- When a feature needs new state on `App`, `Renderer` or `Game`, it gets its own type. It does not add a field to the shell.

**Bad.** `App` (`internal/adapters/inbound/ebiten/app.go:106`) carries 111 fields for every dialog, overlay and HUD strip. Each new screen widens it and every method can reach every field.

**Good.** A feature struct owns its fields, is constructed by the shell, and has its own tests:

```go
type tradeDialog struct {
	sh    tradeShell // consumer-owned, <=8 methods (deps.go)
	open  bool
	offer offerDraft
	scroll int
}

func newTradeDialog(sh tradeShell) *tradeDialog { return &tradeDialog{sh: sh} }
```

The shell keeps one field `trade *tradeDialog` and forwards the entry points. Nothing else in `App` touches `offer` or `scroll`.

---

# 4. O — Open/Closed

**In Go.** Extend by adding a value to a table, or a type that satisfies an interface. Do not edit a switch in a central file to add a feature.

**Rule.**

- Registries are filled explicitly at the composition root (`cmd/*`) or in the owning package's constructor. Never from `init()`.
- A dispatch over string prefixes may exist once per boundary. A new feature registers a handler; it does not add a case.

**Bad.** `screenUIs` (`ui_state.go:15`) is filled by 11 `init()` calls to `registerScreenUI`, so the set of screens depends on file presence and import order. `applyClick` (`app.go:527`) is a 23-arm prefix switch: every new button edits the same function in the same file.

**Good.** The constructor receives the tables:

```go
func NewApp(sim Simulation, screens map[screenID]screenUIFunc, routes []clickRoute) *App

type clickRoute struct {
	prefix string
	handle func(id string)
}
// applyClick: exact-id map first, then the first route whose prefix matches.
```

Each feature file exports a `screenUI` function and a `clickRoute` value; the constructor lists them in one place.

---

# 5. L — Liskov

**In Go.** Anything that satisfies an interface honours its doc contract: same preconditions, same results, same errors. An optional-capability assertion `x.(interface{ Foo() })` is a second interface and needs its own contract.

**Rule.** An assertion for an optional capability is allowed only when all of these hold:

- the feature degrades gracefully without the capability;
- the capability is at most 3 methods and documented as optional;
- it is not inside a hot per-frame loop;
- it is not used to reach a method every real implementation has. That method belongs on the right small reader instead.

**Acceptable.** `tradeSource` (`trade_sources.go`): the trade dialog is absent when the simulation has no trade session.

**Not acceptable.** Asserting from `Simulation` to a wider interface to reach a method every simulation has. Add the method to the small reader whose consumer needs it and make the type satisfy that.

---

# 6. I — Interface Segregation

**In Go.** An interface is a consumer's list of calls. The provider never sees it.

**Rule.**

- At most 8 explicit methods. Compose larger needs by embedding; an embedded interface counts as 1.
- Add a method to the interface whose consumer calls it, never to the widest interface nearby.

**Bad.** `Simulation` (`renderer.go:29`) has 32 methods, consumed alike by the renderer, the HUD, the wizard and the inspectors. Every fake in a test must implement all 32.

**Good.** Small interfaces, each owned by the file that uses it:

- `WorldReader{World}`
- `Clock{Tick, Date, Activity}`
- `Placement{CanPlaceAt, ReplacesAt, AutoFacingAt, CanZoneAt}`
- `Commander{Enqueue}`

A consumer that needs two of them embeds both. `Simulation` shrinks to an embedding of these (at most 8 entries) and then to nothing once every consumer names its own.

---

# 7. D — Dependency Inversion

The mandatory rule is `HEXAGONAL_ARCHITECTURE.md` §6, with `GO_BEST_PRACTICES.md` §5, §6 and §73: ports are consumer-owned, and concrete types are constructed only at the composition root.

This document adds one point. Feature sub-packages (`ebiten/<sub>`, `application/game/<sub>`) are consumers. They define what they need from the root, the root constructs them, and a sub-package never imports the root. The guard's `imports` rule enforces that edge.

In a draw or tick path the host interface's methods take values, not pointers to the caller's locals: an interface call hides the callee from escape analysis, so a `*game.WorldView` passed through one moves the whole frame's world to the heap (ARCH-004 wave 4.3: two allocations per frame until `render/env.Lists` passed the slices instead). Pass the slice or the field the method reads; benchmark `-benchmem` and `-gcflags=-m=2` after adding a host method.

---

# 8. Measurable Limits

| Rule id | Limit | What is measured | Rationale |
| --- | --- | --- | --- |
| `methods` | ≤ 40 per type | Methods declared on a type, across all files of its package | About one `go doc` screen. Only four types exceed it today. |
| `fields` | ≤ 25 per struct with ≥ 5 methods | Fields of a struct that has behaviour; DTO read models are exempt by construction | A struct you cannot hold in your head is two structs. |
| `iface` | ≤ 8 explicit methods | Methods written in an interface; an embedded interface counts 1 | The largest interface in the repo besides `Simulation` is `io.ReadWriteCloser` class. |
| `lines` | ≤ 12,000 non-test lines per package | Non-test Go lines | 3× the third-largest package, so only the two god packages are over it. |
| `files` | ≤ 60 per package | Non-test Go files | Same reasoning as `lines`. |
| `init` | 0 | `func init()` declarations | Registration is explicit (§4). |
| `global` | 0 | Package-level `var` assigned at runtime; `sync` and `atomic` types are exempt | See §9. |
| `imports` | forbidden edges | Domain → adapters, application, platform, ports, `ebiten`, `os`. Application and ports → adapters. Outbound → inbound. `inbound/ebiten/<sub>` → the `ebiten` root. `application/game/<sub>` → the `game` root | `HEXAGONAL_ARCHITECTURE.md` §31 made executable. |

**Budget file.** `internal/arch/solid_budget.txt` holds lines of the form `rule key ceiling  # note`. Each line records today's measured number as a ceiling.

- Growth past a ceiling fails.
- A stale ceiling fails with a message "lower it to N". Lower the line, in the same commit.
- A paid debt whose line is still present fails. Delete the line.
- Never raise a line. If a feature needs more room, split the type (§10).

Run `make arch` before every commit that touches a type, an interface or an import.

---

# 9. Mutable Package-Level State

**Allowed.**

- `sync.Once*`, `sync.Pool`, `sync.Map` and atomic caches, and a cache whose mutex or `Once` is declared in the same `var (…)` group (the guard reads the group as guarded).
- Immutable lookup tables, never assigned after declaration.
- Interface assertions: `var _ I = (*T)(nil)`.
- The one documented process-wide GPU canvas state (`gfx`), used on the main goroutine only and listed in the budget file.

**Not allowed.** Any other variable assigned at runtime. That covers `curUIScale`, `tracePass`, `traceVia`, `canvasTrace`, `drawTrace`, `appBackground`, `appWindow` and `animalMarkTrace`. Each moves onto the type that owns it, or becomes a constructor option.

A test hook becomes a field on its owner, nil in production. It is not a global that a test swaps and restores.

---

# 10. How to Split a God Type

1. Pick one feature cluster. Files already named `*_ui.go` or `*_draw.go` mark the seams.
2. List the fields and methods only that cluster touches (`grep -l 'a\.field'`). A field used by two clusters stays on the shell.
3. Declare `type X struct` holding those fields.
4. Write the consumer-owned interface X needs from the shell or simulation, at most 8 methods, in X's own `deps.go`.
5. Add `NewX(deps)` and call it from the root constructor.
6. The root keeps one field `x *X` and one forwarding method per entry point. Delete each forwarder as its callers are repointed to `x`.
7. Move the cluster's tests with it, in the same commit. Integration tests that drive the whole shell stay at the root.
8. Lower the budget lines in the same commit.

A cluster may become a sub-package of the god type's package when it owns state no other cluster writes and everything it needs from the root fits behind consumer-owned roles in its `deps.go`: each role at most 8 methods, composed by embedding, values only on tick and draw paths (§7). The root builds it, keeps one field, and implements the roles with small adapter types (at most 8 methods each) in `<sub>_host.go`, or beside the root code they wrap when the package's file budget is full; the sub-package never imports the root. New features of a package at its ceiling go into such a sub-package, never into the package itself. Keep the sub-package concise: one concept, several small types if the concept needs them, never one package per type and no `utils`. State that two clusters write stays on the root, behind a role. Keep the sub-package's own types under the limits too: a feature's entry points go on a component type of their own, not on the sub-package's root type. Examples: `internal/adapters/inbound/ebiten/<feature>` (ARCH-005), `internal/application/game/care` (SIM-061…065), `internal/application/game/mind` (SIM-066).

---

# 11. What NOT to Do

- No interface per struct (`GO_BEST_PRACTICES.md` §5, `HEXAGONAL_ARCHITECTURE.md` §37).
- No `utils`, `helpers` or `common` packages (`GO_BEST_PRACTICES.md` §3, §88).
- No package per struct (`HEXAGONAL_ARCHITECTURE.md` §14).
- No provider-owned "full" interface that mirrors a concrete type.
- Do not split a function only to meet a line count (`GO_BEST_PRACTICES.md` §38).
- Never raise a budget line to make the guard pass.
- No forwarding wrapper that lives past the commit series that repoints its callers.
- No abstraction for a single implementation that crosses no package boundary.

---

# 12. Relation to Other Documents

- `GO_BEST_PRACTICES.md` §3, §4–§7, §38, §73–§74, §88: naming, interfaces, constructors, function size, API ownership, lint rules.
- `HEXAGONAL_ARCHITECTURE.md` §6, §14, §26, §31, §37: consumer-owned ports, package boundaries, injection, architecture lint, smells.

This document owns only the Go reading of SOLID and the numeric limits in §8. No rule is restated here, only referenced; where this text and those documents disagree on a dependency rule, they win.
