# TOKEN-001: usage guards and scorecard

Scope: agent tooling only, no game code. Prompted by Spotify's Portal `shunt` article (2026-09-03). We took its lesson (hooks get obeyed, advisory rules don't) but not Portal or its 350-line rule: full Reads over 350 lines were only 3.8% of our tool output. No HUD: nothing here is player-facing.

## Changed
- `.claude/settings.json` (new): the hooks below, and `CLAUDE_CODE_AUTO_COMPACT_WINDOW=300000`.
- `.claude/hooks/read_guard.py`: denies whole-file Reads over 500 lines and `cat`/`sed -n` commands printing over 150 lines of repo files. Limits add up across a command. Fails open.
- `.claude/hooks/agent_model_guard.py`: denies a `general-purpose` or untyped spawn with no `model`.
- `.claude/agents/Explore.md`: `Explore` on Haiku, locating code only.
- `scripts/token_report.py`: scorecard (`--last`, `--since`) and guard dry-run (`--replay`).
- Docs: `AI_TOKEN_OPTIMIZATION.md` §32a and "not built" notes, `PARALLEL_WORK.md` §5a and §6, one rule in root `AGENTS.md`.

## Decisions
- **Guard narrowed from the plan.** The plan denied every plain `cat`/`sed -n` view. The replay showed that would deny 8,300 Bash calls (29%), with 71% of them under 5k chars, where Read prints the same text and a denial only adds a call. At 150 lines it denies 1,683 (5.8%) and still covers 17.9% of tool output. Cost: short shell views still skip package `AGENTS.md` loading.
- **No `CLAUDE_CODE_SUBAGENT_MODEL`.** An art agent launched without a model would silently leave Opus. The hook forces a choice instead.
- **Root `AGENTS.md` rule added** (not in the plan). In auto mode the harness tells agents to read with `cat` and `sed -n`, so without this each agent would waste a call learning the rule.

## Verified
- `python3 .claude/hooks/test_hooks.py`: 25 tests pass.
- Fresh headless Haiku sessions:
  - untyped Agent, `sed -n 1,200p` and a whole-file Read of `tick.go` were denied with the right messages;
  - a ranged Read passed;
  - `Explore` ran all 9 calls on Haiku and answered correctly;
  - Edit after a ranged Read got past the read check;
  - Read loaded `combat/AGENTS.md`, `sed` did not load `craft/AGENTS.md`.
- Replay: 30 sampled Bash denials were all long dumps. File sizes in a replay are today's.
- This session started before the settings file existed, and its own hooks did not visibly fire. New sessions load them.

## Not verified
- Whether `CLAUDE_CODE_AUTO_COMPACT_WINDOW` is honoured from project settings or applies to subagents; the docs don't say. Read it from the scorecard.
- Agents may move to `head`, `tail` or `awk` to get around the guard. The report lists those commands, so a shift will show.

## Scorecard
Baseline: last 400 transcripts on 2026-11-02 (30,995 API calls). After about a week of normal use, run `python3 scripts/token_report.py --since 2026-11-03` and fill in Actual. Compare per-call and share metrics, since totals depend on how much work was done. Expected values are estimates to check, not targets.

| Metric | Baseline | Expected | Actual |
| --- | --- | --- | --- |
| Cache reads per API call, all surfaces | 226k | 180-205k | |
| Cache reads per API call, main sessions | 464k | 200-300k | |
| Cache reads per API call, subagents | 192k | 170-185k | |
| Opus share of subagent cache reads | 92% | 50-65% | |
| `Explore` agents on Opus | 72 (2.9% of all cache reads) | 0 | |
| `sed` + `cat` share of tool output | 44% | 25-30% | |
| `grep` share of tool output | 17% | About the same | |
| `Read` share of tool output | 9.2% | Higher, as shell reads move here | |
| Median peak context, top 50 workers | 417k | 300k or less if compaction applies to subagents; otherwise unchanged | |
| Guard denials, share of `Read` + Bash calls | None; replay 5.8% before agents adapt | Under 3% | |

| Step | Expected effect | Basis | Confidence |
| --- | --- | --- | --- |
| Require a model choice | 20-30% of all cache reads move off Opus; total tokens unchanged | Non-art Opus subagents are 63% of cache reads; assumes a third to half doesn't need Opus | Medium |
| `Explore` on Haiku | About 3% of cache reads move to Haiku | `Explore` on Opus is 2.9% today | High, but small |
| Read guard | 3-8% fewer cache reads | The guard covers 19.9% of tool output (17.9% Bash, 2.0% Read); agents still need part of that | Medium after the replay |
| Earlier compaction | 8-12% fewer cache reads (main sessions only) | Main sessions are 26% of cache reads; 300k should bring 464k down to about 200k, minus re-reads | Medium |

Overall: about 10-20% fewer tokens, with about a quarter of cache reads moving to cheaper models. Spend should fall roughly 20-30% if Sonnet costs about 60% of Opus; that price is an assumption, so look up current prices when filling in Actual.

Watch (no baseline number): bugs or rework traced to Sonnet or Haiku work, lost detail after compaction in long lead sessions, legitimate commands blocked by the guard.

## Follow-ups
- Rollbacks: `READ_GUARD=off` for the guard; delete the `env` entry for compaction.
- If the scorecard justifies it: a lower `bashOutputMaxChars` (12k trims about 6% of Bash output) and trimming oversized package `AGENTS.md` files (`combat` 1,827 words, `ebiten` 1,699, target 300-800).
- Subagents start at about 39k tokens. `syncClaudeAiPlugins: false` in `.claude/settings.local.json` hides claude.ai-synced plugins for this workspace and may shrink that. It is the user's call, because it removes those skills here. Not measured.
