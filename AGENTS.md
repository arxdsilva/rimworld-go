# rimworld-go — root agent instructions

Read only what the current task needs.

## Where things are
- `.agents/README.md` indexes every spec, checklist, gap analysis, ledger, skill, template and hook.
- At task start, Cursor and Codex open `.agents/skills/rimworld-go-router/SKILL.md`; Claude lists the skills itself.
- Specs are in `.agents/specs/`, the two checklists in `.agents/backlog/`, gap analyses in `.agents/gaps/`.
- History: `.agents/handoffs/INDEX.md` and the family ledgers beside it.
- `.cursor/` holds only `mcp.json`; `.claude/` only settings, the Explore agent and skill links.

## Non-negotiables
- Dependencies point inward: adapters → application → domain.
- The domain stays headless (pixels, textures, camera and animation live in presentation adapters) and deterministic.
- One authoritative simulation owner; no goroutine per pawn or job.
- External mutations enter as typed commands; UI and rendering never mutate domain state.
- Code that depends on an adapter, or an adapter that depends on the application, uses a small single-purpose consumer-owned interface; concrete types are built only at the composition root (`.agents/specs/HEXAGONAL_ARCHITECTURE.md` §6).
- Size limits (≤40 methods per type, ≤25 fields on a type with behaviour, ≤8 methods per interface, ≤12k lines or 60 files per package), no `init()` registration, no runtime-assigned package vars: `.agents/specs/SOLID_GO.md` §8–10, enforced by `make arch`. Lines in `internal/arch/solid_budget.txt` only shrink; a package at its ceiling grows through a sub-package (§10).
- Search before creating a shared primitive: this repo, then the standard library, then a well-known library (`.agents/specs/GO_BEST_PRACTICES.md` §86).
- Third-party libraries only in cmd, adapters and platform, never in domain, application or pkg; every `go.mod` module is in `THIRD_PARTY_NOTICES.md`.
- No global RNG or wall-clock time in authoritative gameplay.
- No global scans: use indexes, batching, sleeping and invalidation.
- Animals decide every 60 ticks through the bucket index (`internal/application/game/animal_index.go`, `animal_needs.go`), never a per-tick scan of all creatures (skill `rimworld-go-ai-jobs`, which also locates biome and nature-event data).
- Need-, threat- and threshold-driven behaviour starts and stops at different thresholds (or has a minimum duration or retry cooldown) and ships with a no-flap test (`internal/application/game/sleep_test.go`).
- Run the relevant tests and benchmarks, always with `-race` (`make test`, CI, one-off `go test`), inside CI's 10-minute `-race` budget; tests needing real art index it lazily (`realArt`/`testArt`/`atlasArt`). Never weaken a test to pass CI.
- No proprietary code, assets, text or balance data; the user-approved exceptions are in `.agents/specs/CLEAN_ROOM.md`.
- Remove unused assets, resources and their loaders in the change that makes them unused (no fallback nobody draws; an alias lives only while something reads it); `make art-check` fails on orphans not listed with a reason in `assets/metadata/orphan_allow.txt` (ART §25).
- Grass and plants (ground plants, bushes, crops, trees) never have a contour or outline.
- When work meets an item in a `.agents/backlog/` checklist, the art spec or the screens spec, update its checkbox in the same change: `[x]` all acceptance criteria met, `[-]` partial (note what is missing), `[!]` blocked (note why).
- Player-observable behaviour ships with a read-only HUD/UI representation (via `Snapshot()`/read models) in the same change, or the handoff says why none exists (`.agents/specs/MAIN_UI_SCREENS_SPECIFICATION.md` §0).
- Screens, windows, overlays and HUD go through skill `rimworld-go-ui-screens`; sprites, icons, UI chrome, palettes and asset tooling through `rimworld-go-art-assets`; every new or changed creature through `rimworld-go-creatures` (all four generated sounds, local MOSS by default, ElevenLabs only with approved spending).
- Multi-area tasks follow `.agents/specs/PARALLEL_WORK.md`: every plan has its Parallel workers section (§0); contract first, disjoint file ownership, lead-only shared config; at most 30 agents, bounded by `make agent-budget` (§5); reuse agents, stop each once its work is committed (§5a).
- Every scoped task appends one section to its family ledger in `.agents/handoffs/` and one row to `INDEX.md` (`.agents/templates/HANDOFF.md`).
- Read long files with Read `offset`/`limit`; a hook denies shell views over 150 lines and whole-file Reads over 500 (`.agents/specs/AI_TOKEN_OPTIMIZATION.md` §32a). Name a `model` when spawning a `general-purpose` agent (`PARALLEL_WORK.md` §6).

## Repository overrides
- Agents may stage and commit their own changes here (no `--force`, no `--no-verify`, no rewriting published history). While agents share the checkout, commit with `git commit -m … -- <paths>`.
- Never add Co-Authored-By or Generated-with trailers to commits or PRs.
- All work happens on `main`: no branches, worktrees (`isolation: "worktree"`) or pull requests unless the user asks. Push and tag only on explicit request.
- Releases (tags, packaging, release notes from commit bodies): `.agents/specs/RELEASES.md`.
