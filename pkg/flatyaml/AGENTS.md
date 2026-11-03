# flatyaml

Reads the YAML subset our tools generate for sprite metadata, atlases and palettes. The game's art catalog and the tools that read those files (`cmd/atlasgen`, `cmd/normalgen`) all parse through it.

## Allowed dependencies

- The standard library only, like all of `pkg/`.

## Why this isn't a library (GO_BEST_PRACTICES §86, step 3)

Measured 2026-11-03 against `go.yaml.in/yaml/v3` v3.0.6 over the real tree, on an M2 Max with a warm disk cache. The swap had to add at most 20% and at most 100 ms to the boot index:

| | ours | yaml.v3 |
| --- | --- | --- |
| Parsing the 6,893 sprite files | 8.6 ms, 139k allocs (4.6 ms, 27k allocs with this package) | 116.5 ms, 1.4M allocs |
| One boot index (`art.Index`, single-threaded) | 146 ms | about 261 ms: +115 ms, +79% |

Both read all 7,236 files alike, so the reason is speed alone. Measure again before reconsidering: `BenchmarkParseMetaAll` and `BenchmarkIndex` in `art`.

## Contracts

- Don't add YAML features here. If a file needs more than `Lines` reads, it is time to measure a library again, not to grow this package.
- Comment lines are skipped at any indentation, so they never reach an atlas's embedded metadata.
- A value keeps a trailing `# comment`. Callers that allow one strip it (`cmd/normalgen`).
