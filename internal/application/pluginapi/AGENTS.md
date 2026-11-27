# internal/application/pluginapi

## Purpose
The versioned contract between plugins and the host: the plugin API version a `plugin.json` declares (`api`, apart from the game version) and the closed set of capabilities it may ask for (`capabilities`). Leaf: standard library only.

## Owns
- `Version`, `MinVersion` (both 1 today): a plugin outside `[MinVersion, Version]` does not load.
- `Capability` (13 names: query, commands, events, world, defs, validate, ui, incidents, jobs, stats, health, recipes, worldgen) and `Set` (a bitset; `Has`, `Names`).
- `Parse`: names to a `Set`, reporting unknown and repeated names.

## Contracts
- An unknown name never becomes a `Capability`; the zero value is no capability (`pluginapi_test.go`).
- Refusal rules live in `application/modding/order.go` `CheckManifest` (`TestCheckManifestAPIAndCaps`, `TestResolveRefusesNewerAPI`): api outside the range or an unknown name refuses the plugin, a repeated name warns. A plugin with neither field is legacy: `query`, `commands`, `events` and the warning "declares no plugin API version; treated as 1".
- Enforcement is in `adapters/outbound/starlarkhost/caps.go` (deny by default; `TestCapsTableComplete`).

## Notes
- A new capability: add it last in `names` and the const block, add its hook or ctx rows to `starlarkhost/caps.go`, the table in `application/scripting/doc.go`, and the Plugins detail test (`TestDetailAPICaps`). Raising `Version` is a decision for the whole API, not a per-feature edit.
