# internal/application/modding

## Purpose
The plugin pipeline (PLG-*, `.agents/gaps/MODDING_GAP_ANALYSIS.md`): manifests, load order, definition files, templates, patches, typing into frozen per-class books, and provenance of every problem. Pure: no file access, no third-party import, no encoding/json (it receives decoded trees through `Source`).

## Owns
- `modding.go`: the contract (`Manifest`, `File`, `Plugin`, `Source`, `Problem`, `Placed`, `Order`, `Def`, `Touch`, `Op`).
- `order.go`: `CheckManifest`, `Resolve` (requires/after/before/conflicts, cycles, version warnings, a reason per position).
- `defs.go` `CollectDefs`, `inherit.go` `Inherit`, `patch.go` `ParseOps`/`Apply`, `tree.go` (deep copy, equal, merge, paths).
- `class_backstory.go`, `class_recipe.go`: the class codecs (`Encode*`, `Decode*`); recipe behaviour flags are refused on plugin recipes and fixed on core ones.
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
