# internal/application/modding

## Purpose
The plugin pipeline (PLG-*, `.agents/gaps/MODDING_GAP_ANALYSIS.md`): manifests, load order, definition files, templates, patches, typing into frozen per-class books, and provenance of every problem. Pure: no file access, no third-party import, no encoding/json (it receives decoded trees through `Source`).

## Owns
- `modding.go`: the contract (`Manifest`, `File`, `Plugin`, `Source`, `Problem`, `Placed`, `Order`, `Def`, `Touch`, `Op`).
- `order.go`: `CheckManifest`, `Resolve` (requires/after/before/conflicts, cycles, version warnings, a reason per position).
- `defs.go` `CollectDefs`, `inherit.go` `Inherit`, `patch.go` `ParseOps`/`Apply`, `tree.go` (deep copy, equal, merge, paths).
- `class_backstory.go`, `class_recipe.go`, `class_trait.go`: the class codecs (`Encode*`, `Decode*`); recipe behaviour flags are refused on plugin recipes and fixed on core ones; a trait's note and thought tables (code keyed on core traits) are refused on plugin traits and kept from core on patched core ones. `class_research.go`: research projects decode after recipes (a plugin project's unlocks name the run's recipes); a patch may change a core project's numbers and words, never its prerequisites or unlocks; a plugin project needing an unknown or left-out project, or on a prerequisite loop, is left out (`rsResolve`).
- `class_item.go`: item defs; a plugin item may set `mass` (1 to 100000 g, default 1000), `stack_limit` (1 to 1000, default 1000) and `trader_stock` (0 to 1000, default 0); a core patch may not change them.
- `class_incident.go`: incident defs. Optional data fields, on plugin defs and core patches alike (absent: today's behaviour): `min_points` (0 to 100000, fewest threat points), `points_pct` (0 to 1000, percent of the points spent), `season` (`any`, `winter`, `not_winter`) and `target` (`map`, `world`). `fill` and `under_raid` are structure, not data: not moddable, an `unknown field`. A plugin incident stays a nature incident, and `like` refuses the five storyteller threat faces (hive swarm, predator pack, manhunter pack, raid, mech raid) and `rival_raid`. A threat def's `weight` is its share of the threat die (the raid face's weight is the filler's), not a draw weight in the nature pool.
- `build.go`: `Build` (the pipeline), `Catalog`, `keepCore`, `Classes`, `Errors`.

## Depends on
Domain books (`domain/pawn/backstory`, `domain/economy/recipe`) and their key helpers (`skill.Key`, `backstory.BitKey`/`TagKey`, `item.ParseKind`, `construction.Parse`, `work` and `item` stuff keys).

## Contracts
- Output never depends on file-system or map order: same plugins, any permutation, same catalog and problems (tests shuffle).
- Core defs come first in core order and keep their slots: a patch that removes or breaks one is refused with a problem (saves keep indexes; `keepCore`).
- A bad def or patch drops only itself; a manifest or dependency error stops the plugin and its dependants. Adapter read problems are reported for every enabled plugin.
- A new moddable class: a codec file (`class_<name>.go`, round-trip test `Decode(Encode(core))` deep-equal), one `classes()` entry in `build.go` (core trees + decode into its book on `Catalog`), its book on `game.Content`, its keys in `savefmt.Keys` on save with a remap on load. `keepCore` and the pipeline are class-generic.

## Tests
`order_test.go` (determinism under shuffles), `defs_test.go`, `inherit_test.go`, `patch_test.go`, `tree_test.go`, `class_backstory_test.go` (round trip), `build_test.go` (core kept, unknown class, read problems, release tags). The shipped example plugin is built in `adapters/outbound/modloader/example_test.go`.

## Scripting (PLG-060)
- Plugins' `scripts/**.star` run in Starlark behind `application/scripting.Host`; the runtime is `adapters/outbound/starlarkhost`. Hooks, ctx queries and commands are documented in the `scripting` package comment and `mods/example_frontier/scripts/README.md`.
- Hooks run on the simulation owner in load order; scripts act only through typed commands (`ScriptMessage`, `ScriptSpawn`) via Enqueue; `rand` is a pure function of seed, tick, plugin and call. A script that errors or passes its step budget is turned off for the run with a problem.
