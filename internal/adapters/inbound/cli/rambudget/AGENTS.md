# internal/adapters/inbound/cli/rambudget

## Purpose
Behind `cmd/rambudget` and `make agent-budget` (PLAY-019; `.agents/specs/PARALLEL_WORK.md` §1): sizes the parallel-agent batches from free memory and the measured peaks.

## Owns
`budget.go` (peaks, reserve, `budget`), `run.go` (`Main`, output).

## Depends on
`platform/sysinfo` for memory and cores.

## Contracts
`TestBudget` pins the reserve, the per-kind maximum, the 30-agent cap and the capture kind sitting between light and art.

## Notes
Re-measure the peaks with `/usr/bin/time -l` when packages grow (figures in the PLAY-019 handoff).
