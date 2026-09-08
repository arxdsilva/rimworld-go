# Documentation Index

All paths below are relative to this `.agent/` folder unless noted.

| Need | Read |
|---|---|
| Product/game scope | `MASTER_SPEC.md` |
| Hexagonal architecture and repo boundaries | `HEXAGONAL_ARCHITECTURE.md` |
| Go standards/performance/concurrency | `GO_BEST_PRACTICES.md` |
| Efficient AI context usage | `AI_TOKEN_OPTIMIZATION.md` |
| Per-task AI handoffs | `handoffs/` |
| Task-specific playbooks (start here for any new task) | `skills/rimworld-go-router/SKILL.md` |
| Global contributor rules | `../AGENTS.md` |

Recommended agent loading order:

1. task;
2. `AGENTS.md`;
3. package `AGENTS.md`;
4. interfaces/tests;
5. relevant system doc/ADR;
6. only then broader documents.
