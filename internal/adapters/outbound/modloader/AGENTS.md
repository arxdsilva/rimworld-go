# internal/adapters/outbound/modloader

## Purpose
Finds plugins on disk and reads them into the plain trees `application/modding` works on (PLG-002, PLG-004). Implements `modding.Source`.

## Owns
- `loader.go`: `Loader`, `New(roots...)`, `Plugins()`: one folder per plugin under each root; `plugin.json` (strict, 64 KiB), `defs/**.json`, `patches/**.json`, `lang/<locale>.json`, `scripts/**.star` (256 KiB each) and `scripts/**.wasm` into `Plugin.Wasm` (PLG-063; 1 MiB each, `scripting.MaxWasmBytes`: a compiled binary, so larger than the 256 KiB Starlark source cap); a `.wasm` outside `scripts/` is not read; sorted by id then folder.
- `roots.go`: `DefaultRoots()` (the shipped `mods/` found like the assets, then `<user config>/rimworld-go/mods`), `Open` (a `modding.Source` maker for wiring).

## Contracts
- Every read goes through an `os.Root` per plugin: a path or symlink leaving the folder is a problem, never a read. Caps: 4 MiB per file, 2,000 data files (scripts, `.wasm` and textures included) and 32 MiB of data per plugin (`TestPluginDataTotalIsCapped`).
- An unreadable manifest still lists the plugin (id and name = folder name) with its problem; JSON errors carry line and column.
- No global state, no goroutines.
- The manifest's `capabilities` list is capped at 32 entries (`maxCapabilities`, `loader.go`); a longer list is a problem. The length of one name is not capped here (follow-up, not yet filed).

## Tests
`loader_test.go` (temp-dir fixtures: caps, symlinks, bad JSON, ordering), `example_test.go` (the shipped `mods/example_frontier` builds with no problem; disabled it changes nothing).
