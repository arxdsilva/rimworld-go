---
name: rimworld-go-documentation
description: Write or update rimworld-go package AGENTS.md files, ledger sections, interface summaries and, rarely, ADRs and system docs, without duplicating canonical docs.
---

# Agent documentation

Which document owns what: `.agents/README.md` (the only index). Never copy long canonical sections into another file; cite them by number.

## Package `AGENTS.md`

Follow `.agents/templates/PACKAGE_AGENTS.md` (purpose, owned concepts, allowed and forbidden dependencies, contracts, invariants, hot paths, determinism, tests, benchmarks). Target 300–800 words; only what is local to the package.

## Handoff

Append a section to the family ledger, per `.agents/templates/HANDOFF.md`. Decisions and results only, never a reasoning transcript.

## Interface summary

For a cross-agent dependency, write a compact signature and invariant summary rather than copying whole packages.

## ADR and system docs

`docs/adr` and `docs/systems` are empty today. Write an ADR only for a cross-cutting decision that no spec section records (context, decision, consequences, alternatives, determinism / performance / persistence impact); a system doc only for a subsystem whose behaviour no gap analysis or spec already describes. Decisions, not tutorials.
