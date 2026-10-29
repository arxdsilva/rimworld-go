# art package

## Package Purpose
The generated sprite catalog: atlas pages, sprite metadata, tints, normal maps and memory accounting. It draws nothing and knows nothing of the game. Moved out of the ebiten root in ARCH-003 wave 3.1 (`art/`, 1,866 lines).

## Owned Concepts
- `Catalog` (21 methods, 12 exported) with `Load`, `Index`, `Locate`; `Catalog.ReadBack func(string) bool` tells the catalog which keys the renderer reads back (plants, grass).
- `Sprite`, `Meta` (`ParseMeta`), `Atlas` (`ParseAtlas`), `AnimalExt`, `TintKey`, `MemSnapshot`.
- `Facing` (`FaceSouth` and the other facings), `FacingFor`, `Origin`, `Mix64`.
- Tint: `ParseTints`, `TintNRGBA`, `TintTexel`, `MaskImage`, `MaskCoverage`, `DecodeNRGBA`; the `TintShader` (a `sync.OnceValues` over the embedded tint shader).
- `Deflater` (a `compress/flate` helper) and the `container/list` LRU and `sync` pool state for memory accounting.

## Allowed Dependencies
- Ebitengine (`ebiten`) for `*ebiten.Image` and `*ebiten.Shader`.
- `platform/assetroot` (locating `assets/`).
- Standard library: `embed`, `image`, `image/png`, `compress/flate`, `container/list`, `sync`, `sync/atomic`, `os`, `path/filepath`.

## Forbidden Dependencies
- Domain and application packages (`internal/domain`, `internal/application`): art imports neither.
- The ebiten root package and `cmd/`.
- Feature packages and `render/*`: the catalog is below every draw layer.

## Public Contracts
- `Load(root)` and `Index(root, preferFiles)` return `*Catalog`; `Locate()` finds the assets directory.
- `Catalog` lookups return `*Sprite` and `Meta`; `Origin(m, sx, sy, tile)` maps sprite pivots to screen offsets.
- Atlas parsing is pure (`ParseAtlas`, `ParseMeta`, `ParseTints`) so tests run without files.

## Invariants
- Sprite keys are stable strings; metadata and atlases are the single source (`assets/metadata`).
- `TestNoOrphanSprites` (run inside this package) guards the asset list; see `assets/metadata/orphan_allow.txt`.
- No package-level mutable state except `sync`-guarded pools and onces.

## Hot Paths
- Catalog lookups run once per drawn sprite from `render/*`; `Origin` and `FacingFor` are pure arithmetic.
- Loading and tint baking are startup or cache-miss work, not per frame. Not benchmarked in this package.

## Determinism Rules
- Presentation only. `Mix64` is a pure hash for sprite variant choice; it never reads the simulation's RNG or the clock.

## Save/Mod Impact
- Reads `assets/` (metadata, atlases, generated sprites) at load time. Writes nothing.

## Tests
- 3 source files, 3 test files, 23 `Test*` functions: `TestParseSpriteAtlas`, `TestLoadAtlasCutsSpritesLikeFiles`, `TestLoadAtlasPrefersPagesOverIndividualFiles`, `TestLoadAtlasRejectsBrokenAtlases`.
- Coverage 88 % (ARCH-003).
- Tests needing the real art index use the lazy `realArt`/`testArt`/`atlasArt` helpers in the root package.

## Benchmarks
- None in the package.

## Known Design Constraints
- `ReadBack` is set by the root's `loadArtCatalog`/`indexArtCatalog` wrappers.
- A race was reported once in this package's tests right after the move (ARCH-003 watch note); CI `-race` is the guard.
