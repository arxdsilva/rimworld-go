# Parallel work rule: fan out by default

Split work across **parallel subagents** whenever a task touches two or more of the independent areas below. Do a single-area change, or one under ~150 lines, directly. The skill `parallel-work` (`skills/parallel-work/SKILL.md`) has the checklist and the agent prompt template.

| Area | Paths |
| --- | --- |
| domain | `internal/domain/**` |
| application / simulation | `internal/application/**` |
| ports | `internal/ports/**` |
| renderer / UI adapter | `internal/adapters/inbound/ebiten/**` |
| outbound adapters | `internal/adapters/outbound/**` |
| composition root | `cmd/**` |
| art pipeline | `assets/source/**` and its generated output (`assets/generated/**`, `assets/metadata/**`) |
| content | `content/`, `mods/` |
| docs / specs | `.cursor/*.md`, `docs/`, `README.md` |

## 1. The lead goes first, alone

| Do (lead, before launching agents) | Don't |
| --- | --- |
| Write the **shared contract**: `.cursor/*.md` spec sections (e.g. ART §, UI screen §), application read models (`internal/application/game/*View`), consumer-owned interfaces in the consuming package (`HEXAGONAL_ARCHITECTURE.md` §6), ports in `internal/ports/**`, typed commands (`internal/domain/sim/command`), asset names, metadata and attachment schema, save format | Let agents invent interfaces each other depend on |
| **Stub every new hook** so every agent compiles from the first minute | Let a consumer wait for a producer to compile |
| **Add every dependency** (`go get`, `go mod tidy`) | Let two agents touch `go.mod` / `go.sum` |
| Make all **shared-config** edits: `go.mod`, `go.sum`, `Makefile`, `.gitignore`, CI workflows, staticcheck config, `cmd/**` (flags, scenarios), `assets/palettes/*.yaml`, `assets/source/blender/artlib/{contracts.py,stage.py}`, `README.md`, `.cursor` checklists, specs and handoffs | Hand shared config to an agent |
| Build shared binaries (`bin/game`, `bin/benchmark`) and record baselines (benchmarks, captures) | Let agents rebuild `bin/*` concurrently |

## 2. Disjoint ownership

| Do | Don't |
| --- | --- |
| Give every agent an explicit list of **owned files**, **forbidden files**, the **contract sections** it follows, its **verification commands**, and a **word-capped report** format | Launch an agent with "fix whatever you find" |
| Split along the hexagon: one agent per layer/package area. Inside a large package, split by file or function and name them | Give two agents the same file |
| Use `isolation: "worktree"` when ownership can't be disjoint; the lead merges | Hope concurrent edits to one file work out |

## 3. Inside an agent

| Agents do | Agents never |
| --- | --- |
| TDD (red, then green). Never lower a touched package's coverage; aim for ≥70% (`go test -cover`). The `AGENTS.md` non-negotiables still apply: dependency direction, headless deterministic domain, no global RNG or wall clock in the sim, typed commands only | `go get` / `go mod tidy`, edit shared config, `git add/commit/push`, edit checklists or handoffs (the lead writes them from reports) |
| Run **targeted** checks only: `go test -race -count=1 -cover ./<owned pkgs>/...`, `go vet ./<owned pkgs>/...`; for art, `make art FAMILIES=<family>` and `make art-check` | Run `make all`, `make test`, `go test ./...`, `make art` without `FAMILIES`, `make screenshots`/`make clips`, or `go build -o bin/...` (others are mid-change; build to the scratchpad if needed) |
| Report failures in other agents' files | "Fix" another agent's files or tests, or weaken a test to pass |
| Develop against the contract and stubs when a producer isn't done yet | Block idle waiting; finish with stand-ins and report |

## 4. Phases follow dependencies

contract (spec §, read models, stubs) → **producers in parallel** (domain/sim, pathing, Blender art, pure logic) → **consumers** (renderer, UI, outbound adapters; may start early against stubs) → **lead verification** → docs, checklists, handoffs, README media.

## 5. Coordination and verification

| Do | Don't |
| --- | --- |
| Relay cross-agent facts through the lead (`SendMessage`) | Let agents coordinate implicitly through files |
| Make contract changes as the lead, additive only, and broadcast them to every affected agent | Let one agent change a shared contract |
| After each phase, run the gates below, then apply the merge checklist in `skills/rimworld-go-review-integration/SKILL.md` | Declare done from agent reports alone |
| Route fixes back to the owning agent | Patch an active agent's files behind its back |
| Keep ≤ 5 agents running at once | Spawn more because "it's faster" |

**Gates after each phase:**

1. `go build ./...`
2. `make fmt` (must print nothing), `make vet`, `make lint`
3. `go test -race -count=1 ./...`
4. Determinism: `go test -count=3 -run 'Determin|SameSeed|Hash' ./internal/...`
5. `make art-check` when art changed
6. `bin/benchmark` against the baseline when the sim or a hot path changed
7. A visual review (`bin/game -capture …`, frame strips, contact sheets) for presentation or art work

**Plan mode pauses background agents.** Avoid entering it while agents run; if you must, resume each one with `SendMessage` afterwards.

## 6. Pick the cheapest competent model

Every subagent runs on the **cheapest model that can do its job well**. Choose per agent, not per task.

| Tier | `Agent` `model:` | Use for |
| --- | --- | --- |
| 1 | `haiku` | Mechanical, fully specified work: repo searches and inventories (Explore), running captures and benchmarks and collecting the output, checklist and handoff edits from facts the lead supplies, fixtures |
| 2 | `sonnet` | Standard implementation against a written contract: renderer and UI features, screens, Blender family edits against an exact spec, outbound adapters, unit tests, docs prose |
| 3 | `opus` | Hard reasoning: tick order and determinism, pathing and movement arbitration, reservations and jobs, save migrations, concurrency and workers, interpolation and timing math, architecture and plan review |

| Do | Don't |
| --- | --- |
| Start at the lowest tier that can do the job | Pick a higher tier "to be safe" |
| Escalate one tier when an agent fails its gate twice or reports it is stuck; relaunch with the reason | Keep retrying a stuck agent on the same tier |
| State the model and a one-line reason in every launch, and add a Model column to every phase table | Leave the model implicit (it then inherits the lead's) |
| Keep the lead on the session's model | Use other models (e.g. `fable`) unless the user asks for it |
