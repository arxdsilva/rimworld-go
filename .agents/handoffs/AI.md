# AI handoffs

Families: AI. Each section condenses one task's handoff (agent guidance, its layout and tooling). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## AI-001-agents-folder
One `.agents/` folder for every agent tool · 2026-11-06 · 76d605d04
- Changed: all agent guidance moved from `.cursor/` and `.claude/` into `.agents/` (specs, backlog, gaps, handoffs, skills, templates, hooks), indexed by `.agents/README.md`. `.cursor/` keeps only `mcp.json`; `.claude/` keeps settings, the Explore agent and one symlink per skill (Claude finds skills only there). Cursor's `.mdc` rules are gone (every point lives in root or a skill).
- Changed: root `AGENTS.md` 12,680 → 5,238 bytes: one line per non-negotiable, pointing to its canonical home. New specs `CLEAN_ROOM.md` (the ban plus one table of every user-approved data exception) and `RELEASES.md`.
- Changed: 351 handoffs and 16 plans became 20 family ledgers (1.57 MB → 768 KB with `INDEX.md`); each section is headed by its old file stem, citations point to `<LEDGER>.md#<stem>`, plan parts that still own an open checklist row are verbatim. `scripts/handoff_index.py` rebuilds `INDEX.md`.
- Changed: specs say each rule once (MASTER drops the greenfield layout, backlog and branching; HEX shows the real tree; AI_TOKEN drops unbuilt tools; PARALLEL drops worktrees). 18 skills → 16 (persistence-replay merged into determinism, bootstrap deleted); descriptions 3,757 → 2,622 bytes, all ≤ 160 characters. 62 package `AGENTS.md` follow `.agents/templates/PACKAGE_AGENTS.md` (2,912 → 2,317 lines).
- Decisions: user, 2026-11-06: folder `.agents/`; handoffs and plans rolled up per family, originals deleted (`git show 76d605d04:.agents/handoffs/<stem>.md`); checklists moved, content unchanged. The seven repo facts in the Claude memory moved into root, PARALLEL_WORK §3/§5a, SOLID_GO §10, the ebiten and assets `AGENTS.md` and the testing skill.
- Decisions: guard `internal/arch/agents_test.go` (`make arch`): no `.cursor/` (except mcp.json) or `.claude/hooks` paths outside `.agents/`; links, backticked `.agents/` paths and ledger anchors resolve; `.claude/skills` matches `.agents/skills`; every ledger section has its title line and `INDEX.md` row; root `AGENTS.md` within `internal/arch/agents_budget.txt` (only shrinks).
- Limits: ledgers came out at 720 KB, not the 350 KB planned: the verbatim open plan parts (PLAY018, NG1) and dense SIM/PLAY history were kept over the target. A sample check kept a median 81% of each original's IDs and numbers; the misses are line numbers, § references and benchmark tables.
- Changed: ART §37 no longer names the removed `TestPendingArtToolsLackArt` step.
- Limits: the big specs (ART, MAIN_UI, MASTER, GO) were trimmed only of restated rules; their bodies are unchanged, so `.agents/specs` is 344 KB (was 352 KB).
- No HUD/UI representation: agent guidance only, no player-observable change. No checklist row covers this work. No determinism, save or mod impact (Go changes are comments plus the guard test).
- Open: Cursor skill discovery is unverified; Cursor reaches skills through the router named in root.

## AI-002-no-tool-folders
Agent files live only in `.agents/`; the repo keeps no tool folder · 2026-11-08 · pending
- Changed: after `.claude/` was removed (`660f5d4c2`), `TestSkillLinksMatch` gave way to `TestNoToolFolders`, which fails if `.claude`, `.cursor`, `.codex`, `.windsurf` or `.gemini` exists at the root. `TestNoStaleAgentPaths` no longer lets a line name Cursor's folder for its `mcp.json`.
- Changed: AGENTS.md, `.agents/README.md` §8 and §9, AI_TOKEN_OPTIMIZATION §32a, PARALLEL_WORK, `docs/DEVELOPING.md` and `tools/blender-mcp/README.md` say each tool is pointed at `.agents/` from its user's own settings. Every tool now opens the router skill at task start.
- Decisions: the user's rule (2026-11-08) is that anything agent-related lives in `.agents/`. The root AGENTS.md budget went down to 5,198 bytes.
- Open: the root `.mcp.json` (Blender and ElevenLabs servers) still sits outside `.agents/`, because that is where Claude Code reads project MCP servers; moving it is the user's call.
