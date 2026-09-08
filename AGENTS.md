# rimworld-go — Root AI/Contributor Instructions

Read only what the current task needs.

All canonical specs, rules, and agent-facing data live in [`.cursor/`](.cursor/DOCUMENT_INDEX.md):
- `.cursor/MASTER_SPEC.md` — product/system scope.
- `.cursor/HEXAGONAL_ARCHITECTURE.md` — dependency direction, ports/adapters, repository ownership.
- `.cursor/GO_BEST_PRACTICES.md` — mandatory Go engineering rules.
- `.cursor/AI_TOKEN_OPTIMIZATION.md` — context/token discipline.
- `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` — visual production, sprites, icons, UI chrome, and asset pipeline (load only when that work is in scope; playbook: `.cursor/skills/rimworld-go-art-assets/SKILL.md`).
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` — per-screen layout, navigation, commands, and states (load only when a player-facing screen is in scope; playbook: `.cursor/skills/rimworld-go-ui-screens/SKILL.md`).
- `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md` — ordered vanilla gameplay feature backlog.
- `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` — ordered player-facing UI/UX backlog.
- `.cursor/handoffs/` — per-task AI handoff records.
- `.cursor/skills/` — task playbooks; start with `.cursor/skills/rimworld-go-router/SKILL.md`.
- `.cursor/rules/` — Cursor always-on recognition (art direction, UI screens).

Non-negotiable rules:
- Dependencies point inward: adapters → application → domain.
- Domain must remain headless and deterministic.
- One authoritative simulation owner; no goroutine-per-pawn/job.
- Search before creating shared primitives.
- All external mutations enter as typed commands.
- UI/rendering never mutates domain state directly.
- Any code that depends on an adapter, or an adapter that depends on the application layer, must depend on a small, consumer-owned interface (the behavior it needs), never the concrete type directly — construct concrete types only at the composition root. This is SOLID's Dependency Inversion Principle; keep each such interface single-purpose per the Interface Segregation Principle. See `.cursor/HEXAGONAL_ARCHITECTURE.md` §6.
- No global RNG or wall-clock time in authoritative gameplay.
- Avoid global scans; use indexes, batching, sleeping, and invalidation.
- Run relevant tests and benchmarks.
- Do not weaken tests to pass CI.
- Do not copy proprietary code, assets, text, or balancing data.
- Every scoped task must leave a concise handoff in `.cursor/handoffs/`.
- Whenever work satisfies (fully or partially) an item in `.cursor/RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`, `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`, `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md`, or `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`, update that item's checkbox in the same change: `[x]` only when its full acceptance criteria are met, `[-]` when partially done (state what's missing in a short trailing note), `[!]` when blocked (state why). Never leave completed work unreflected in the checklists.
- Any newly implemented domain/application behavior that is player-observable per `.cursor/MASTER_SPEC.md` or `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` must ship with a corresponding HUD/UI representation (read-only, via `Snapshot()`/read models, never a direct domain reference) in the same change. If that representation is a named screen, follow `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` via skill `rimworld-go-ui-screens`. If no sensible on-screen representation exists yet (e.g. a pure internal primitive with no player-facing effect), state why in the handoff instead of adding one.
- Player-facing screens, windows, overlays, HUD layout, and navigation must follow `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md` via skill `rimworld-go-ui-screens`. Load only the sections that skill names for the screen. Use `.cursor/RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` for ordered backlog/acceptance. Screens consume read models and emit typed commands; they never mutate domain state. Skip that spec for headless simulation-only work with no screen.
- Player-visible sprites, icons, UI chrome, renderer presentation, palettes, pivots, atlases, and asset-pipeline tooling must follow `.cursor/ART_DIRECTION_AND_ASSET_PIPELINE.md` via skill `rimworld-go-art-assets`. Load only the sections that skill names for the asset family. Domain stays headless: pixels, texture handles, camera, and animation stay in presentation adapters. Never trace, extract, or reconstruct proprietary RimWorld art. Skip that spec for headless simulation-only work with no visual output.

## Repository-specific override: commits

In this repository only, the global rule forbidding agents from running `git add`/`git commit` does not apply. Agents may stage and commit their own changes here, following the message style and safety rules in the global git workflow guidance (no `--force`, no `--no-verify`, no rewriting published history, never push without explicit request).
