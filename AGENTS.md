# rimworld-go — Root AI/Contributor Instructions

Read only what the current task needs.

All canonical specs, rules, and agent-facing data live in [`.cursor/`](.cursor/DOCUMENT_INDEX.md):
- `.cursor/MASTER_SPEC.md` — product/system scope.
- `.cursor/HEXAGONAL_ARCHITECTURE.md` — dependency direction, ports/adapters, repository ownership.
- `.cursor/GO_BEST_PRACTICES.md` — mandatory Go engineering rules.
- `.cursor/SOLID_GO.md` — SOLID applied to Go here: type, interface and package size limits, no `init()` registration, no mutable package state; enforced by `internal/arch/solid_test.go` (`make arch`), budgets in `solid_budget.txt` only shrink.
- `.cursor/AI_TOKEN_OPTIMIZATION.md` — context/token discipline.
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` — visual production, sprites, icons, UI chrome, and asset pipeline (load only when that work is in scope; playbook: `.cursor/skills/rimworld-go-art-assets/SKILL.md`).
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` — per-screen layout, navigation, commands, and states (load only when a player-facing screen is in scope; playbook: `.cursor/skills/rimworld-go-ui-screens/SKILL.md`).
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` — ordered vanilla gameplay feature backlog.
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` — ordered player-facing UI/UX backlog.
- `.cursor/handoffs/` — per-task AI handoff records.
- `.cursor/skills/` — task playbooks; start with `.cursor/skills/rimworld-go-router/SKILL.md`.
- `.cursor/rules/` — Cursor always-on recognition (art direction, UI screens, creatures).
- `.cursor/PARALLEL_WORK.md` — how multi-area tasks fan out to parallel subagents (playbook: `.cursor/skills/parallel-work/SKILL.md`).

Non-negotiable rules:
- Dependencies point inward: adapters → application → domain.
- Domain must remain headless and deterministic.
- One authoritative simulation owner; no goroutine-per-pawn/job.
- Search before creating shared primitives.
- All external mutations enter as typed commands.
- UI/rendering never mutates domain state directly.
- Any code that depends on an adapter, or an adapter that depends on the application layer, must depend on a small, consumer-owned interface (the behavior it needs), never the concrete type directly — construct concrete types only at the composition root. This is SOLID's Dependency Inversion Principle; keep each such interface single-purpose per the Interface Segregation Principle. See `.cursor/HEXAGONAL_ARCHITECTURE.md` §6.
- Type, interface and package size limits (`.cursor/SOLID_GO.md` §8: ≤40 methods per type, ≤25 fields on a type with behaviour, ≤8 methods per interface, ≤12k lines / 60 files per package) are enforced by `go test ./internal/arch/`. Never raise a line in `internal/arch/solid_budget.txt`, only lower or delete one; register nothing from `init()`; no package-level var assigned at runtime outside the documented exceptions.
- No global RNG or wall-clock time in authoritative gameplay.
- Avoid global scans; use indexes, batching, sleeping, and invalidation.
- Animals make full decisions every 60 ticks through the bucket index (`internal/application/game/animal_index.go`, `animal_needs.go`); never scan all creatures per tick. Biome data lives in `internal/domain/world/biome`, nature-event definitions in `internal/domain/incident`.
- Every need-, threat- or threshold-driven behaviour must start and stop at different thresholds (hysteresis), or have a minimum duration or retry cooldown, so actors don't flap between two activities. Any new one ships with a no-flap test (see `internal/application/game/sleep_test.go`).
- Run relevant tests and benchmarks. Tests always run with `-race` (`make test`, CI, agents, one-off `go test`); keep the suite inside CI's 10-minute `-race` budget (tests that need real art index it lazily: `realArt`/`testArt`/`atlasArt` in the ebiten package).
- Do not weaken tests to pass CI.
- Do not copy proprietary code, assets, text, or balancing data.
  Exception (ENG-003, extended 2026-10-26): the social/host-system feature set and the medical system (MED-*, `.cursor/MEDICAL_GAP_ANALYSIS.md`) use published RimWorld wiki numbers by explicit user decision; names and prose stay original.
- Remove unused assets and resources in the same change that makes them unused: sprites, sounds, fonts, metadata, palette entries, vendor packs, generated files and their loaders. Never keep a fallback asset nobody draws. An alias may stay only while something still reads it; delete it in the change that removes the last reader. `make art-check` fails on orphans not listed with a reason in `assets/metadata/orphan_allow.txt`.
- Grass and plants (ground plants, bushes, crops, trees) never have a contour or outline.
- Every scoped task must leave a concise handoff in `.cursor/handoffs/`.
- Multi-area tasks follow `.cursor/PARALLEL_WORK.md` (skill `parallel-work`): contract first, disjoint file ownership, lead-only shared config, phased verification.
- Whenever work satisfies (fully or partially) an item in `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`, `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`, or `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`, update that item's checkbox in the same change: `[x]` only when its full acceptance criteria are met, `[-]` when partially done (state what's missing in a short trailing note), `[!]` when blocked (state why). Never leave completed work unreflected in the checklists.
- Any newly implemented domain/application behavior that is player-observable per `.cursor/MASTER_SPEC.md` or `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` must ship with a corresponding HUD/UI representation (read-only, via `Snapshot()`/read models, never a direct domain reference) in the same change. If that representation is a named screen, follow `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` via skill `rimworld-go-ui-screens`. If no sensible on-screen representation exists yet (e.g. a pure internal primitive with no player-facing effect), state why in the handoff instead of adding one.
- Player-facing screens, windows, overlays, HUD layout, and navigation must follow `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` via skill `rimworld-go-ui-screens`. Load only the sections that skill names for the screen. Use `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` for ordered backlog/acceptance. Screens consume read models and emit typed commands; they never mutate domain state. Skip that spec for headless simulation-only work with no screen.
- Every new or changed animal/creature follows skill `rimworld-go-creatures` (`.cursor/skills/rimworld-go-creatures/SKILL.md`): roster row in Go and the art mirror, rig art with full sprite set, and **all four generated sounds** (`call`, `attack`, `hurt`, `die`) through `cmd/sfxgen` with user-approved spending — never ship a creature silent, on placeholder voices only, or as a copy of a RimWorld-invented creature.
- Player-visible sprites, icons, UI chrome, renderer presentation, palettes, pivots, atlases, and asset-pipeline tooling must follow `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` via skill `rimworld-go-art-assets`. Load only the sections that skill names for the asset family. Domain stays headless: pixels, texture handles, camera, and animation stay in presentation adapters. Never trace, extract, or reconstruct proprietary RimWorld art. Skip that spec for headless simulation-only work with no visual output.

## Repository-specific override: commits

In this repository only, the global rule forbidding agents from running `git add`/`git commit` does not apply. Agents may stage and commit their own changes here, following the message style and safety rules in the global git workflow guidance (no `--force`, no `--no-verify`, no rewriting published history, never push without explicit request).

## Repository-specific override: work on `main`

This repository has a single developer (arxdsilva). All work happens directly on `main`: do not create feature branches, worktrees (`isolation: "worktree"`), or pull requests unless the user asks for one. This overrides any "branch first when on the default branch" guidance. Parallel subagents (`.cursor/PARALLEL_WORK.md`) share the main checkout with disjoint file ownership. Pushing and tagging still need an explicit request.

## Releases

Pushing a `v*` tag (`git tag v0.1.0 && git push origin v0.1.0`) runs `.github/workflows/release.yml`. The workflow tests the code, then builds Linux (amd64), Windows (amd64, no cgo) and macOS (universal arm64+amd64, cgo) binaries. Each binary is packaged by `scripts/package_release.sh` with the runtime assets, and the archives are published as a GitHub release; a tag with a `-` (e.g. `v0.2.0-rc1`) becomes a prerelease. The binary finds `assets/` next to itself (`internal/platform/assetroot`), and the menu shows the version from `-ldflags "-X main.version=<tag>"`. Any new runtime asset directory must be added to the packaging script.
