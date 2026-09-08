# Documentation Index

| Need | Read |
|---|---|
| Product/game scope | `MASTER_SPEC.md` |
| Hexagonal architecture and repo boundaries | `HEXAGONAL_ARCHITECTURE.md` |
| Go standards/performance/concurrency | `GO_BEST_PRACTICES.md` |
| Efficient AI context usage | `AI_TOKEN_OPTIMIZATION.md` |
| Global contributor rules | `AGENTS.md` |

Recommended agent loading order:

1. task;
2. `AGENTS.md`;
3. package `AGENTS.md`;
4. interfaces/tests;
5. relevant system doc/ADR;
6. only then broader documents.
