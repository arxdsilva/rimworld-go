# internal/adapters/inbound/cli/contentlint

## Purpose
Behind `cmd/contentlint` (PLG-040, `.agents/gaps/MODDING_GAP_ANALYSIS.md`): runs `modding.Build` over the installed plugins without a window and prints the load order, per-class counts and every problem with its provenance. It also compiles the loaded plugins' `scripts/**.star` (PLG-060): a syntax or name error is an error at `<file>:<line>`.

## Owns
`contentlint.go` (`Main`, flags `-mods`, `-enable`, `-game`, `-q`; `scriptProblems` compiles scripts with the game's dialect, `scriptOptions`, which `scripts_test.go` keeps equal to `starlarkhost.Options`).

## Depends on
`internal/application/modding`. The plugin source arrives as an `Open` func; `cmd/contentlint/main.go` passes `modloader.New`.

## Contracts
Exit 0 clean or warnings only, 1 on any error, 2 on bad flags or an unreadable folder. With no `-enable`, every installed plugin is enabled.
