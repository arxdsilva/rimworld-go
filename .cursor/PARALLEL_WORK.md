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

## 0. Every plan includes its parallel work (user decision, 2026-10-29)

A plan for a multi-area task (plan mode, a plan file, or a written phase plan) is incomplete until it carries a **Parallel workers** section. Write it before presenting the plan, not after approval:

- **Areas and contract.** The areas the task touches, and what the lead writes alone before any agent starts: interfaces, read models, typed commands, stubs, shared config.
- **Waves in dependency order.** One table per wave with these columns:
  - id;
  - model and a one-line reason (§6);
  - job;
  - owned files;
  - forbidden files;
  - the targeted test commands the agent runs.
- **RAM.** The `make agent-budget` check and each wave's size (§1).
- **Integration.** The lead's gates after each wave (§5) and the commit points.
- **Assets.** Art, sound and creature agents whenever assets are involved: art on `opus` with `rimworld-go-art-assets`, creatures per `rimworld-go-creatures`.

A single-area task, or one under ~150 lines, states that in one line instead. The skill `parallel-work` has a template for the section.

## 1. The lead goes first, alone

**RAM first (user decision, 2026-10-25).** Before launching any agent, and again before each later phase, run `make agent-budget` (`cmd/rambudget`). It reads the machine's total and free memory and prints the most agents of each kind that fit, from measured peaks:
- heavy: a full `go test -race` of the ebiten package, 4.7 GiB because its tests decode the art; only the lead runs it, alone;
- art: a Blender render, a game run on a 250+ map, or an fps probe, about 3 GiB;
- capture: a game run on a small map for screenshots, about 2.2 GiB (one at a time);
- light: an agent's targeted `go test -race -run …` of its own tests, about 1 GiB with the compile.

Agents never run a full package that loads the art. Never run more agents than it allows. The 15-agent cap in §5 is a ceiling, not a target. Start heavy agents a few minutes apart rather than all at once, and tell every agent to run tests with `GOFLAGS=-p=2`. If an agent reports slow tests or the machine swaps, re-check before launching more.

| Do (lead, before launching agents) | Don't |
| --- | --- |
| Run **`make agent-budget`** and size the batch to it | Launch a batch without checking free RAM |
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
| Run **targeted** checks only, with `GOFLAGS=-p=2`: `go test -race -count=1 -cover ./<owned pkgs>/...`, `go vet ./<owned pkgs>/...`; for art, `make art FAMILIES=<family>` and `make art-check` | Run `make all`, `make test`, `go test ./...`, `make art` without `FAMILIES`, `make screenshots`/`make clips`, or `go build -o bin/...` (others are mid-change; build to the scratchpad if needed) |
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
| Keep ≤ 15 agents running at once, and never more than `make agent-budget` allows | Spawn more than the work has disjoint areas for, or more than free RAM holds |
| Re-run `make agent-budget` before each phase and when an agent reports slow tests | Start every heavy agent in the same minute |

**Gates after each phase:**

1. `go build ./...`
2. `make fmt` (must print nothing), `make vet`, `make lint`
3. `go test -race -count=1 ./...`
4. Determinism: `go test -count=3 -run 'Determin|SameSeed|Hash' ./internal/...`
5. `make art-check` when art changed
6. `bin/benchmark` against the baseline when the sim or a hot path changed
7. A visual review (`bin/game -capture …`, frame strips, contact sheets) for presentation or art work

**Plan mode pauses background agents.** Avoid entering it while agents run; if you must, resume each one with `SendMessage` afterwards.

## 5a. Agent lifecycle: reuse, then clean up (user decision, 2026-10-30)

Every spawned agent stays in Claude Code's agent list for the rest of the session. A long programme that spawned around 200 agents left the list full of finished ones. No tool removes a finished run from that list, so keep the count low at the source and close agents as soon as their work is in.

| Do | Don't |
| --- | --- |
| Send follow-ups, fixes and the same area's next wave to the agent that already owns it (`SendMessage` resumes it with its context) | Spawn a fresh agent for a fix, a relay or a re-run in an area an existing agent owns |
| Keep one docs agent and one journey agent per programme, resumed each wave | Spawn a new docs or journey agent every wave |
| Do one-fact lookups, small edits and single test runs yourself | Spawn an agent for work under about 30 lines or one search |
| Stop each agent with `TaskStop` once its work is integrated and committed and you won't resume it | Leave finished agents open "just in case" across commits |
| Before each wave commit, stop every agent of that wave you won't resume; at the end of a programme, stop them all | Carry a programme's agents into unrelated work |
| When the list has grown large (about 50 or more) and the work is committed, tell the user that a new session starts with a clean list | Claim `TaskStop` removes entries: it only marks them stopped |

Notes:
- **Stopped agents stay listed.** A stopped agent stays listed but is not running, and messaging it resumes it, so stopping costs nothing if you later need it.
- **Clearing rows by hand:** the user can clear a failed or stopped row sooner with `x` in `/tasks`.
- **Explore and Plan agents are one-shot.** Size their prompt to finish in one go.
- **Running-agent limit:** Claude Code allows 20 running subagents by default (`CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`). This rule's limit of 15 or fewer, and `make agent-budget`, stay stricter.

## 6. Pick the cheapest competent model

Every subagent runs on the **cheapest model that can do its job well**. Choose per agent, not per task.

| Tier | `Agent` `model:` | Use for |
| --- | --- | --- |
| 1 | `haiku` | Mechanical, fully specified work: repo searches and inventories (Explore), running captures and benchmarks and collecting the output, checklist and handoff edits from facts the lead supplies, fixtures |
| 2 | `sonnet` | Standard implementation against a written contract: renderer and UI features, screens, outbound adapters, unit tests, docs prose |
| 3 | `opus` | Hard reasoning: tick order and determinism, pathing and movement arbitration, reservations and jobs, save migrations, concurrency and workers, interpolation and timing math, architecture and plan review. **All art** (see below) |

**Art always runs on the best model (user decision, 2026-10-25).** Every agent that makes, changes or visually reviews art (palettes, UI chrome colours, icons, sprites, Blender families, contact-sheet and capture review) runs on `opus` and loads skill `rimworld-go-art-assets`. Art is never put on a lower tier to save cost; split art out of a mixed agent instead. Only mechanical capture runs (running commands, collecting files) may stay on `haiku`.

| Do | Don't |
| --- | --- |
| Start at the lowest tier that can do the job | Pick a higher tier "to be safe" |
| Escalate one tier when an agent fails its gate twice or reports it is stuck; relaunch with the reason | Keep retrying a stuck agent on the same tier |
| State the model and a one-line reason in every launch, and add a Model column to every phase table | Leave the model implicit (it then inherits the lead's) |
| Keep the lead on the session's model | Use other models (e.g. `fable`) unless the user asks for it |
