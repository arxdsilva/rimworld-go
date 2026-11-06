# DOS-3: `dossier.Panel`

Package `internal/adapters/inbound/ebiten/dossier` (files `panel.go`, `data.go`, `build.go`, `item.go`, `layout.go`, `draw.go`, `tip.go`, `pick.go`, `stats.go`, `scene.go`). `go test -race` passes (coverage 73.7 percent), `go vet` and `make arch` pass.

## API for DOS-4 to DOS-6

```go
func New(r Reader, a Actions, p Prefs, s ports.Sizer, k *uikit.Kit, ic *art.Catalog, sc Scene) *Panel
```
- `Scene` (new, `scene.go`, 3 methods): `AreaOf(id) int`, `FoodPolicies() []game.FoodPolicyView`, `Hour() int`. `Reader` has none of them, so the host fills this too (`Game.AreaOf`, `World().FoodPolicies`, `World().Hour`).
- PaneBody: `Claims`, `Declare(r)`, `Draw(dst, r)`, `Title(k, id)`. Floating: `DeclareFloating(r)`, `DrawFloating(dst, r)` (draws `uikit.DrawPanel` and a 6 px margin; r is the whole card).
- `Select(k, id)` (drops the cache on a change; Claims and Title call it too), `Reset()` (map switch, return to menu: RHUD-503/504; the root must call it), `Invalidate()` (setup edits: next Declare reads the facts and settings again), `Height(docked, w)` (content height, plus the margins when floating), `OpenStats()` (header info button, RHUD-012), `SetOnSetup(func())` (what the gear opens: DOS-5 sets it).
- Order per Update: `kit.Begin`, then `Declare*`, then at draw time `Draw*`. The gear's press is taken before any card widget sees it, then the gear widget `ids.DossierGear` is added last.

## Behaviour worth knowing
- Refresh gate: `HUD.RefreshTenths * 6` Updates (kit host `Updates()`, 60 a second), at once on a new id, `Invalidate`, or a change of Text, layouts (by pointer), colour count, Enabled or Restyle. Colour edits that keep the count are seen at the next refresh or `Invalidate`.
- Tips are built when the cursor has rested 20 Updates; queued through `Kit.Queue`; up to 280 px wide; each line in its tone.
- Selectors use `uikit.Picker` for all three menus (food, area, hour) with a last "Manage..." entry for food (opens `tab.assign`) and area (`Actions.ManageAreas`). `Actions.AreaPicker` is therefore not used.
- An unknown tree key draws a marker box and calls `Hint` once per key.

## Deviations and gaps
- Area selector uses its own menu instead of `Actions.AreaPicker` (so it can end with the manage entry).
- Hour back tints use `picker_back` blended 35 percent toward `tone_warning`, `tone_good`, `side_ally` (no schedule colour tokens exist).
- `notes` is built here (`build.go`: drafted, infection, malnutrition, toxic, frostbite; pregnant, hunts, chill, heat, toxic for animals). `sheet` has no notes builder; move it there if wanted.
- `Scene` is the one addition beyond `deps.go`.
- Not drawn yet: faction emblem, the older-row click (DOS-4, DOS-7).

## Perf
`BenchmarkDossierDeclare`: 0 allocs, about 220 ns. `BenchmarkDossierRefresh`: about 50 us, 495 allocs (at the refresh only). `BenchmarkDossierDraw` (Begin, Declare, Draw into an image): about 100 us, 555 allocs, all inside Ebitengine's recorded draw commands (`text.Draw`, `vector.FillRect`); none of ours.
