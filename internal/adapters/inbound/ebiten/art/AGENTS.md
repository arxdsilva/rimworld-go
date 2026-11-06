# internal/adapters/inbound/ebiten/art

## Purpose
The generated sprite catalog: atlas pages, sprite metadata, tints, normal maps and memory accounting. It draws nothing and knows nothing of the game. Moved out of the ebiten root in ARCH-003 wave 3.1 (`art/`, 1,866 lines).

## Owns
- `Catalog` (21 methods, 12 exported) with `Load`, `Index`, `Locate`; `Catalog.ReadBack func(string) bool` tells the catalog which keys the renderer reads back (plants, grass).
- `Sprite`, `Meta` (`ParseMeta`), `Atlas` (`ParseAtlas`), `AnimalExt`, `TintKey`, `MemSnapshot`.
- `Facing` (`FaceSouth` and the other facings), `FacingFor`, `Origin`.
- Tint: `ParseTints`, `TintNRGBA`, `TintTexel`, `MaskImage`, `MaskCoverage`, `DecodeNRGBA`; the `TintShader` (a `sync.OnceValues` over the embedded tint shader).
- `Deflater` (a `compress/flate` helper) and the `container/list` LRU and `sync` pool state for memory accounting.

## Depends on
- Ebitengine for `*ebiten.Image` and `*ebiten.Shader`.
- `platform/assetroot` (locating `assets/`), `pkg/flatyaml` (sprite metadata, atlases and palettes).

## Must not depend on
- Domain and application packages (`internal/domain`, `internal/application`).
- Feature packages and `render/*`: the catalog is below every draw layer.

## Contracts
- `Load(root)` and `Index(root, preferFiles)` return `*Catalog`; `Locate()` finds the assets directory.
- `Catalog` lookups return `*Sprite` and `Meta`; `Origin(m, sx, sy, tile)` maps sprite pivots to screen offsets.
- Atlas parsing is pure (`ParseAtlas`, `ParseMeta`, `ParseTints`) so tests run without files.
- Reads `assets/` (metadata, atlases, generated sprites) at load time; writes nothing.

## Invariants
- Sprite keys are stable strings; metadata and atlases are the single source (`assets/metadata`).
- `TestNoOrphanSprites` (run inside this package) guards the asset list; see `assets/metadata/orphan_allow.txt`.
- No package-level mutable state except `sync`-guarded pools and onces.

## Hot paths
- Catalog lookups run once per drawn sprite from `render/*`; `Origin` and `FacingFor` are pure arithmetic.
- Loading and tint baking are startup or cache-miss work, not per frame. Not benchmarked here.

## Tests
- 3 source files, 3 test files, 23 `Test*` functions: `TestParseSpriteAtlas`, `TestLoadAtlasCutsSpritesLikeFiles`, `TestLoadAtlasPrefersPagesOverIndividualFiles`, `TestLoadAtlasRejectsBrokenAtlases`.
- Coverage 88 % (ARCH-003).
- `BenchmarkParseMetaAll`, `BenchmarkParseAtlasAll`, `BenchmarkParseTints`, `BenchmarkIndex` run over the real tree. They are what to measure before reconsidering a YAML library (`pkg/flatyaml/AGENTS.md`).
- Tests needing the real art index use the lazy `realArt`/`testArt`/`atlasArt` helpers in the root package.

## Notes
- `ReadBack` is set by the root's `loadArtCatalog`/`indexArtCatalog` wrappers.
- A race was reported once in this package's tests right after the move (ARCH-003 watch note); CI `-race` is the guard.
