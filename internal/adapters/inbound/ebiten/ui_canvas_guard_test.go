package ebiten

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The canvas guard (PLAY-012 B.4, rewritten on go/ast for PLAY-017). UI is
// laid out in logical pixels and drawn through the canvas helpers
// (ui_canvas.go), so it scales; world drawing is positioned by the camera
// in physical pixels. Two bugs shipped because the two mixed: zone labels
// drifted off their zone (a world pass handed physical positions to
// drawText, which wants logical ones) and the colonist-bar portraits
// floated mid-screen (a helper in a world file drew logical positions raw).
// A per-file allow-list could not see either, so the guard works per
// function:
//
//   - Rule 1: only functions listed in canvasExempt (key "file.go:func",
//     with a reason) may draw raw (vector, ebitenutil, DrawImage,
//     DrawTriangles*, Draw*Shader, text.Draw).
//   - Rule 2: a function that converts world positions (worldToScreen*,
//     or the shell's WorldToScreen*)
//     must not call the logical-coordinate draws (drawText, drawIcon, ui*,
//     ...) directly; it goes through worldText / worldIcon, which convert
//     once. A function that divides by uiScale() itself is converting by
//     hand and passes (the pan test checks its arithmetic);
//     worldDrawsLogical lists the rest that are right for another reason.
//   - Rule 3: functions in HUD, inspector and panel files (hudFiles) may
//     not own raw draws and may not call the world pixel helpers
//     (drawSprite, solidRect, ...), which take physical pixels.
//
// Entries that no longer apply fail too, so the lists stay honest. The
// dynamic side of the same guarantee is visual_invariance_test.go.

// canvasExempt are the functions that may draw raw for good, in physical
// pixels: the canvas and its text path, the world renderer (everything the
// camera positions) and the full-screen backgrounds. Key "file.go:func"
// (a method is keyed by its bare name; a closure belongs to the function
// around it). A HUD or panel function never belongs here: draw it through
// the canvas.
var canvasExempt = map[string]string{
	// the canvas itself: logical in, physical out
	"gfx/canvas.go:DrawImage":    "the canvas: logical in, physical out",
	"gfx/canvas.go:FillCircle":   "the canvas: logical in, physical out",
	"gfx/canvas.go:FillPoly":     "the canvas: logical in, physical out",
	"gfx/canvas.go:FillRect":     "the canvas: logical in, physical out",
	"gfx/canvas.go:Image":        "the canvas: logical in, physical out",
	"gfx/canvas.go:Line":         "the canvas: logical in, physical out",
	"gfx/canvas.go:StrokeCircle": "the canvas: logical in, physical out",
	"gfx/canvas.go:StrokeRect":   "the canvas: logical in, physical out",
	"gfx/canvas.go:Triangles":    "the canvas: logical in, physical out",
	"gfx/canvas.go:Triangles32":  "the canvas: logical in, physical out",
	"gfx/text.go:Text":           "the canvas's text path: scaled faces at physical positions",

	// the world renderer
	"renderer.go:drawDaylight":           "world: the sky tint over the map",
	"renderer.go:drawGhostRect":          "world: the architect's drag ghost on tiles",
	"renderer.go:drawGrid":               "world: the no-art tile grid",
	"renderer.go:drawPawn":               "world: the no-art pawn square",
	"renderer.go:drawSelection":          "world: the no-art selection box",
	"renderer.go:drawTerrain":            "world: the no-art ground",
	"renderer.go:drawWorld":              "world: the no-art objects",
	"draw_art.go:drawItemObj":            "world: ground items",
	"draw_art.go:drawPlacementGhost":     "world: the building ghost on tiles",
	"draw_art.go:drawSpriteScaled":       "world: sprite blit at a tile position",
	"sprites.go:drawImage":               "world sprite blit (UI callers use uiImage)",
	"lod.go:drawPlantMark":               "world: plant marks on tiles",
	"lod.go:drawUnlit":                   "world: unlit sprite blit",
	"lod.go:solidRect":                   "world: a tile-space rectangle as a sprite",
	"light.go:drawLit":                   "world: the light pass",
	"light.go:litSource":                 "world: the light pass",
	"anim.go:drawParticles":              "world: particles",
	"anim.go:drawSpritePose":             "world: posed sprite blit",
	"anim.go:drawWorkGlow":               "world: work glow on tiles",
	"animal_figure.go:drawAnimalFigure":  "world: animal figures",
	"pawn_anim.go:drawSnoreFor":          "world: snore marks over sleepers",
	"burial_anim.go:drawCorpseItem":      "world: corpses, graves, pyres",
	"burial_anim.go:drawFlameScreen":     "world: pyre flames",
	"fire_world_draw.go:drawWorldFires":  "world: flames, ash decals",
	"arrival_draw.go:drawArrivals":       "world: drop-pod arrivals",
	"bridge_draw.go:drawBridgeCell":      "world: bridge water shadows and fallback deck",
	"cave_plants_draw.go:drawDuskGlow":   "world: the duskmoss glow discs",
	"crop_batch.go:drawLitBatch":         "world: crop unit and furrow batches",
	"biome_art.go:drawTroughFallback":    "world: trough fallback quad",
	"blood_draw.go:drawStains":           "world: blood batches",
	"dirt_draw.go:drawDirt":              "world: dirt batches",
	"fog_draw.go:drawFogLayer":           "world: fog of war chunks over the map",
	"grass_batch.go:bakeGrassTint":       "world: grass batches",
	"art_memory.go:bakeTint":             "world: tinted sprite texture baked offscreen on the GPU",
	"grass_batch.go:flushGrass":          "world: grass batches",
	"shadow.go:drawShadows":              "world: shadows",
	"gfx/batch.go:Flush":                 "world: shadow triangle batch",
	"shadow_cast.go:castSilhouette":      "world: cast shadows",
	"water_draw.go:drawWaterTile":        "world: water",
	"gfx/worldpass.go:CompositeWorld":    "world: the render-scaled map composited to the screen",
	"paths_draw.go:drawPaths":            "world: path lines",
	"pen_draw.go:penSide":                "world: pen region outline on tiles",
	"zones_draw.go:zoneFill":             "world: zone fill batch",
	"zones_draw.go:drawZones":            "world: zone tiles",
	"zones_draw.go:drawZoneHighlight":    "world: the selected zone's brighter fill and outline on tiles",
	"fence.go:drawFenceVector":           "world: fences on tiles",
	"fence.go:drawForbiddenMark":         "world: forbidden marks on tiles",
	"draw_combat.go:drawMeleeTrail":      "world: combat effects",
	"draw_combat.go:drawTraces":          "world: combat effects",
	"environment.go:drawCentered":        "world: weather sprites over the map",
	"environment.go:drawLightning":       "world: lightning over the map",
	"environment.go:drawWeather":         "world: weather over the map",
	"environment.go:flush":               "world: weather triangle batch",
	"mapfields_overlay.go:drawMapFields": "world: dev map-field layer over the map",

	// overlays anchored to tiles (the UI around them uses the canvas)
	"labels.go:drawBreakClouds":                   "world: storm clouds over colonists (stack counts and icons use the canvas)",
	"labels.go:drawBuildingDamage":                "world: damage bars under buildings",
	"work_progress.go:drawWorkProgress":           "world: work progress bars over the work spots",
	"turret_draw.go:drawRangeRing":                "world: a selected turret's range ring on the map",
	"labels.go:drawFightMarker":                   "world: fight marker over colonists",
	"labels.go:drawHostileMarker":                 "world: hostile chevron over raiders",
	"building_inspector.go:drawBuildingSelection": "world: the selected building's outline on the map (the pane uses the canvas)",
	"room_stats.go:drawRoomOverlay":               "mixed: the room tile tint is world (the hover card uses the canvas)",
	"multiselect.go:drawGroupMarks":               "world: group marks on tiles (the pane and the drag box use the canvas)",
	"order_queue_ui.go:dashed":                    "world: order route over the map (text goes through the canvas)",
	"order_queue_ui.go:drawOrderRoute":            "world: order route over the map",
	"formation.go:drawFormation":                  "world: formation markers on tiles",
	"architect.go:drawZoneGhost":                  "world: zone/drag ghost cells on the map (palette and labels use the canvas)",
	"move_mode.go:drawMoveMode":                   "world: move-mode footprint outline (hints use the canvas)",
	"zone_tools.go:drawZoneToolGhost":             "world: the Expand and Shrink ghost cells on the map (the icon and hint use the canvas)",
	"power_overlay.go:powerDot":                   "world: the power overlay's device marks on tiles",
	"power_overlay.go:drawPowerLink":              "world: power link lines between tiles (the overlay and the placement ghost)",
	"power_overlay.go:drawPowerFrame":             "world: the red frame round a device on no power grid",

	// the globe: physical size inside its logical area
	"globe.go:Draw":                "globe: drawn at physical size inside its logical area (x s); names go through the canvas",
	"globe.go:drawBody":            "globe: the sphere, at physical size",
	"globe.go:drawMarks":           "globe: marks on the sphere",
	"world_ui.go:drawCaravanMarks": "globe overlay: caravan marks drawn on the globe surface",

	// full-screen backgrounds, physical size, unscaled
	"menu_diorama.go:drawMenuDiorama": "full-screen background (physical size, unscaled)",
	"menu_diorama.go:loadMenuDiorama": "full-screen background bake (physical size, unscaled)",
	"setup_backdrop.go:drawPlanet":    "full-screen background (physical size, unscaled)",
	"setup_backdrop.go:drawStars":     "full-screen background (physical size, unscaled)",
}

// worldDrawsLogical are the functions that call worldToScreen* AND
// logical-coordinate draws directly, and are right anyway (a function that
// mentions uiScale() is converting by hand and needs no entry). Key
// "file.go:func", with the reason.
var worldDrawsLogical = map[string]string{
	"move_mode.go:drawMoveMode": "the world position only feeds the raw footprint outline; the hint is drawn at the cursor, which is logical",
}

// logicalDraws are the calls that take LOGICAL coordinates (x the UI
// scale on the way out). A world-pass function calling one with a
// worldToScreen position is the zone-label bug.
var logicalDraws = map[string]bool{
	"drawText": true, "drawIcon": true, "drawIconCS": true, "drawLabel": true,
	"drawLabelClip": true, "drawFitSmall": true, "drawPanel": true, "drawButton": true,
	"debugPrintAt": true,
	"uiFillRect":   true, "uiStrokeRect": true, "uiLine": true, "uiFillCircle": true,
	"uiStrokeCircle": true, "uiFillPoly": true, "uiImage": true, "uiDrawImage": true,
	"uiTriangles": true, "uiTriangles32": true,
}

// worldPixelHelpers take PHYSICAL pixels (they are exempt, world-side
// blits). A HUD function calling one with logical coordinates is the
// portrait bug.
var worldPixelHelpers = map[string]bool{
	"drawSprite": true, "drawSpritePose": true, "drawSpriteScaled": true,
	"solidRect": true, "drawImage": true, "drawUnlit": true, "drawBaked": true,
}

// hudFiles are the HUD, inspector, panel and screen files (globs): all
// logical-pixel UI. They hold no raw draws and call no world pixel helper
// (Rule 3). Files that mix in a world overlay (building_inspector.go,
// room_stats.go, multiselect.go, order_queue_ui.go, architect.go,
// move_mode.go, formation.go, labels.go, world_ui.go) are left out; Rules
// 1 and 2 cover them per function.
var hudFiles = []string{
	"hud.go", "inspector.go", "*_inspector.go", "alerts.go", "status_chips.go",
	"threat_ui.go", "climate_ui.go", "chase_ui.go", "widgets.go",
	"search_ui.go", "animals_ui.go", "archtools.go", "letters.go", "toast.go",
	"screen_*.go", "ui_*.go", "*_ui.go", "portrait.go",
	"hud_*.go", "cmds_*.go", // PLAY-018 HUD regions
}

// hudFilesNot are files the globs above catch that are not HUD files.
var hudFilesNot = []string{
	"world_ui.go",       // globe overlay
	"order_queue_ui.go", // world route
	"building_inspector.go",
}

// hudCallsWorldHelper are HUD functions that call a world pixel helper on
// purpose, with the reason it is right.
var hudCallsWorldHelper = map[string]string{
	"animal_inspector.go:drawAnimalMark": "marks the selected animal on the map with overlay/selection: world tile, physical pixels, like the map",
	"frame_inspector.go:drawFrameMark":   "marks the selected frame's tiles on the map with overlay/selection: world tiles, physical pixels",
	"item_inspector.go:drawItemMark":     "marks the selected stack on the map with overlay/selection: world tile, physical pixels",
	"plant_inspector.go:drawPlantMark":   "marks the selected plant on the map with overlay/selection: world tile, physical pixels",
}

// funcScan is what the guard found in one function (all functions of one
// name in one file, closures included).
type funcScan struct {
	File, Name string
	Raw        []string // raw draws, "line N: vector.FillRect"
	World      []string // worldToScreen* uses
	Logical    []string // logicalDraws calls
	Helpers    []string // worldPixelHelpers calls
	UIScale    bool     // calls uiScale(): converts world positions by hand
}

func (f *funcScan) key() string { return f.File + ":" + f.Name }

// rawMethods are the draw methods that bypass the canvas.
var rawMethods = map[string]bool{
	"DrawImage": true, "DrawTriangles": true, "DrawTriangles32": true,
	"DrawTrianglesShader": true, "DrawTrianglesShader32": true, "DrawRectShader": true,
}

// importNames maps a file's local package names to the raw-draw packages
// it imports (vector, ebitenutil, text).
func importNames(af *ast.File) map[string]string {
	out := map[string]string{}
	for _, im := range af.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		var pkg string
		switch {
		case strings.HasSuffix(p, "/ebiten/v2/vector"):
			pkg = "vector"
		case strings.HasSuffix(p, "/ebiten/v2/ebitenutil"):
			pkg = "ebitenutil"
		case strings.HasSuffix(p, "/ebiten/v2/text/v2"), strings.HasSuffix(p, "/ebiten/v2/text"):
			pkg = "text"
		case strings.HasSuffix(p, "/inbound/ebiten/gfx"):
			pkg = "gfx"
		default:
			continue
		}
		local := path.Base(p)
		if pkg == "text" {
			local = "text"
		}
		if im.Name != nil {
			local = im.Name.Name
		}
		out[local] = pkg
	}
	return out
}

// scanFile reports, per function of af, the raw draws, world conversions,
// logical draws and world pixel helper calls. base is the file's name.
func scanFile(fset *token.FileSet, af *ast.File, base string) []*funcScan {
	pkgs := importNames(af)
	byName := map[string]*funcScan{}
	var order []*funcScan
	get := func(name string) *funcScan {
		if s, ok := byName[name]; ok {
			return s
		}
		s := &funcScan{File: base, Name: name}
		byName[name] = s
		order = append(order, s)
		return s
	}
	at := func(n ast.Node, what string) string {
		return "line " + strconv.Itoa(fset.Position(n.Pos()).Line) + ": " + what
	}
	for _, d := range af.Decls {
		name := "<pkg>" // package-level var/const/type initialisers
		if fd, ok := d.(*ast.FuncDecl); ok {
			name = fd.Name.Name
		}
		var fs *funcScan
		scan := func() *funcScan {
			if fs == nil {
				fs = get(name)
			}
			return fs
		}
		ast.Inspect(d, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					switch pkgs[id.Name] {
					case "vector", "ebitenutil":
						scan().Raw = append(scan().Raw, at(x, id.Name+"."+x.Sel.Name))
					}
				}
				if strings.HasPrefix(x.Sel.Name, "worldToScreen") || strings.HasPrefix(x.Sel.Name, "WorldToScreen") { // the renderer's, or the shell role's (shell.go)
					scan().World = append(scan().World, at(x, x.Sel.Name))
				}
			case *ast.CallExpr:
				switch fn := x.Fun.(type) {
				case *ast.SelectorExpr:
					if id, ok := fn.X.(*ast.Ident); ok && pkgs[id.Name] == "gfx" { // the canvas package: logical in, physical out
						switch fn.Sel.Name {
						case "UIScale":
							scan().UIScale = true
						case "Text", "Icon", "Image", "DrawImage", "FillRect", "StrokeRect", "Line", "FillCircle", "StrokeCircle", "FillPoly", "Triangles", "Triangles32":
							scan().Logical = append(scan().Logical, at(x, "gfx."+fn.Sel.Name))
						}
						return true
					}
					if rawMethods[fn.Sel.Name] {
						scan().Raw = append(scan().Raw, at(x, "."+fn.Sel.Name))
					}
					if id, ok := fn.X.(*ast.Ident); ok && pkgs[id.Name] == "text" && fn.Sel.Name == "Draw" {
						scan().Raw = append(scan().Raw, at(x, "text.Draw"))
					}
					if worldPixelHelpers[fn.Sel.Name] {
						scan().Helpers = append(scan().Helpers, at(x, fn.Sel.Name))
					}
				case *ast.Ident:
					if fn.Name == "uiScale" {
						scan().UIScale = true
					}
					if logicalDraws[fn.Name] {
						scan().Logical = append(scan().Logical, at(x, fn.Name))
					}
					if worldPixelHelpers[fn.Name] {
						scan().Helpers = append(scan().Helpers, at(x, fn.Name))
					}
				}
			}
			return true
		})
	}
	return order
}

// packageFiles are the non-test .go files of dir and of its sub-packages
// (the ebiten tree: a feature that moves into its own package stays
// guarded), as slash paths relative to dir ("hud.go", "gfx/canvas.go").
func packageFiles(t testing.TB, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && (d.Name() == "fontdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// scanPackage scans every non-test .go file of dir and its sub-packages.
func scanPackage(t testing.TB, dir string) []*funcScan {
	t.Helper()
	fset := token.NewFileSet()
	var out []*funcScan
	for _, rel := range packageFiles(t, dir) {
		f := filepath.Join(dir, rel)
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, scanFile(fset, af, rel)...)
	}
	return out
}

// isHUDFile reports base is a HUD / panel file (hudFiles minus hudFilesNot).
func isHUDFile(base string) bool {
	for _, g := range hudFilesNot {
		if g == base {
			return false
		}
	}
	for _, g := range hudFiles {
		if ok, _ := path.Match(g, base); ok {
			return true
		}
	}
	return false
}

// guardViolations applies the three rules (and the stale-entry checks) to
// scans, with the given lists. declared are the package's function names
// (for the lists of helper names).
func guardViolations(scans []*funcScan, declared map[string]bool, files []string,
	exempt, logicalOK, helperOK map[string]string) []string {
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	byKey := map[string]*funcScan{}
	for _, s := range scans {
		byKey[s.key()] = s
	}
	for _, s := range scans {
		k := s.key()
		// Rule 1: raw draws only in exempt functions.
		if len(s.Raw) > 0 && exempt[k] == "" {
			add("rule 1: %s draws raw (draw through ui_canvas.go, or exempt it with a reason if it is world drawing):\n\t\t%s", k, strings.Join(s.Raw, "\n\t\t"))
		}
		// Rule 2: world conversion and logical draws never in one function
		// (unless it divides by uiScale() itself).
		if mixesWorldAndLogical(s) && logicalOK[k] == "" {
			add("rule 2: %s converts world positions (%s) and calls logical draws directly (%s): use worldText / worldIcon (physical in)", k, s.World[0], strings.Join(s.Logical, ", "))
		}
		// Rule 3: HUD files own no raw draws and call no world pixel helper.
		if isHUDFile(s.File) {
			if exempt[k] != "" {
				add("rule 3: %s is a HUD/panel file function exempt from the canvas: HUD code draws through the canvas", k)
			}
			if len(s.Helpers) > 0 && helperOK[k] == "" {
				add("rule 3: %s (HUD/panel) calls world pixel helpers (%s) with logical coordinates: use uiImage / uiFillRect", k, strings.Join(s.Helpers, ", "))
			}
		}
	}
	// Stale entries.
	for k := range exempt {
		if s := byKey[k]; s == nil || len(s.Raw) == 0 {
			add("stale: canvasExempt %q (the function no longer draws raw)", k)
		}
	}
	for k := range logicalOK {
		if s := byKey[k]; s == nil || !mixesWorldAndLogical(s) {
			add("stale: worldDrawsLogical %q (the function no longer mixes the two, or converts by hand)", k)
		}
	}
	for k := range helperOK {
		if s := byKey[k]; s == nil || len(s.Helpers) == 0 || !isHUDFile(s.File) {
			add("stale: hudCallsWorldHelper %q (the function no longer calls a world helper)", k)
		}
	}
	for n := range logicalDraws {
		if !declared[n] {
			add("stale: logicalDraws names %q, which the package no longer declares", n)
		}
	}
	for n := range worldPixelHelpers {
		if !declared[n] {
			add("stale: worldPixelHelpers names %q, which the package no longer declares", n)
		}
	}
	for _, g := range append(append([]string{}, hudFiles...), hudFilesNot...) {
		matched := false
		for _, f := range files {
			if ok, _ := path.Match(g, f); ok {
				matched = true
			}
		}
		if !matched {
			add("stale: hudFiles/hudFilesNot pattern %q matches no file", g)
		}
	}
	sort.Strings(out)
	return out
}

// mixesWorldAndLogical is Rule 2's test: the function takes a world
// position and draws logical coordinates directly, without converting.
func mixesWorldAndLogical(s *funcScan) bool {
	return len(s.World) > 0 && len(s.Logical) > 0 && !s.UIScale
}

// declaredFuncs are the names of the package's functions and methods.
func declaredFuncs(t testing.TB, dir string) (names map[string]bool, files []string) {
	t.Helper()
	names = map[string]bool{}
	fset := token.NewFileSet()
	for _, rel := range packageFiles(t, dir) {
		f := filepath.Join(dir, rel)
		files = append(files, rel)
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				names[fd.Name.Name] = true
			}
		}
	}
	return names, files
}

// TestUIDrawsThroughCanvas is the guard: see the comment at the top of
// this file for the three rules.
func TestUIDrawsThroughCanvas(t *testing.T) {
	scans := scanPackage(t, ".")
	declared, files := declaredFuncs(t, ".")
	bad := guardViolations(scans, declared, files, canvasExempt, worldDrawsLogical, hudCallsWorldHelper)
	if len(bad) > 0 {
		t.Errorf("%d canvas guard violation(s):\n%s", len(bad), strings.Join(bad, "\n"))
	}
}

// guardOn scans one source for the guard's own tests, with empty lists
// except the given ones.
func guardOn(t *testing.T, file, src string, exempt, logicalOK map[string]string) []string {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	scans := scanFile(fset, af, file)
	declared := map[string]bool{}
	for n := range logicalDraws {
		declared[n] = true
	}
	for n := range worldPixelHelpers {
		declared[n] = true
	}
	var out []string
	for _, v := range guardViolations(scans, declared, []string{file}, exempt, logicalOK, nil) {
		if !strings.Contains(v, "pattern") { // the file globs are checked on the real package
			out = append(out, v)
		}
	}
	return out
}

const guardHeader = "package p\nimport (\n\t\"github.com/hajimehoshi/ebiten/v2/vector\"\n\ttext \"github.com/hajimehoshi/ebiten/v2/text/v2\"\n)\n"

func TestGuardRule1RawDraws(t *testing.T) {
	src := guardHeader + `
func hud(dst I) { vector.FillRect(dst, 0, 0, 1, 1, c, false) }
func hud2(dst I) { dst.DrawImage(img, op); dst.DrawTriangles32(v, i, img, o); text.Draw(dst, "x", f, op) }
func world(dst I) { vector.StrokeLine(dst, 0, 0, 1, 1, 1, c, false) }
func clean(dst I) { uiFillRect(dst, 0, 0, 1, 1, c, false) }
`
	got := guardOn(t, "x.go", src, map[string]string{"x.go:world": "world"}, nil)
	if len(got) != 2 || !strings.Contains(got[0], "x.go:hud ") || !strings.Contains(got[1], "x.go:hud2 ") {
		t.Fatalf("violations = %q, want hud and hud2 only", got)
	}
	if !strings.Contains(got[1], ".DrawImage") || !strings.Contains(got[1], ".DrawTriangles32") || !strings.Contains(got[1], "text.Draw") {
		t.Errorf("hud2 should list all three raw draws: %q", got[1])
	}
	// a local variable named vector/text is not the package
	src = "package p\nfunc f() { vector := 1; text := \"\"; _, _ = vector, text }\n"
	if got := guardOn(t, "y.go", src, nil, nil); len(got) != 0 {
		t.Errorf("locals named like packages are not raw draws: %q", got)
	}
}

func TestGuardRule2WorldToScreenThenLogical(t *testing.T) {
	src := guardHeader + `
func label(r R) { sx, sy := r.worldToScreenF(1, 2); drawText(dst, "x", float64(sx), float64(sy), st) }
func icon(r R) { sx, sy := r.worldToScreen(1, 2); drawIcon(dst, img, float64(sx), float64(sy), 8) }
func rect(r R) { sx, _ := r.worldToScreenFoo(1, 2); uiFillRect(dst, sx, 0, 1, 1, c, false) }
func good(r R) { sx, sy := r.worldToScreenF(1, 2); worldText(dst, "x", float64(sx), float64(sy), st) }
func hand(r R) { sx, _ := r.worldToScreen(1, 2); drawText(dst, "x", float64(sx)/uiScale(), 0, st) }
func cursor(r R) { ox, _ := r.worldToScreen(1, 2); vector.StrokeRect(dst, ox, 0, 1, 1, 1, c, false); drawText(dst, "hint", mx, my, st) }
func hudOnly() { drawText(dst, "x", 1, 2, st) }
`
	got := guardOn(t, "x.go", src, map[string]string{"x.go:cursor": "world outline"}, map[string]string{"x.go:cursor": "the hint is at the logical cursor"})
	if len(got) != 3 {
		t.Fatalf("violations = %q, want label, icon and rect (hand converts itself, cursor is listed)", got)
	}
	for i, n := range []string{"x.go:icon", "x.go:label", "x.go:rect"} {
		if !strings.Contains(got[i], n) || !strings.Contains(got[i], "rule 2") {
			t.Errorf("violation %d = %q, want rule 2 for %s", i, got[i], n)
		}
	}
}

func TestGuardRule3HUDFiles(t *testing.T) {
	if !isHUDFile("hud.go") || !isHUDFile("screen_menu.go") || !isHUDFile("plant_inspector.go") || !isHUDFile("climate_ui.go") {
		t.Error("HUD/panel files not recognised")
	}
	if isHUDFile("gfx/canvas.go") || isHUDFile("renderer.go") || isHUDFile("building_inspector.go") || isHUDFile("order_queue_ui.go") {
		t.Error("canvas, world and mixed files are not HUD files")
	}
	src := "package p\nfunc bar(r R) { r.solidRect(dst, 1, 2, 3, 4, c); drawSprite(dst, k) }\nfunc ok(r R) { uiFillRect(dst, 1, 2, 3, 4, c, false) }\n"
	got := guardOn(t, "hud.go", src, nil, nil)
	if len(got) != 1 || !strings.Contains(got[0], "rule 3") || !strings.Contains(got[0], "hud.go:bar") ||
		!strings.Contains(got[0], "solidRect") || !strings.Contains(got[0], "drawSprite") {
		t.Fatalf("violations = %q, want rule 3 for bar", got)
	}
	// the same call in a world file is fine
	if got := guardOn(t, "shadow.go", src, nil, nil); len(got) != 0 {
		t.Errorf("world files may call the helpers: %q", got)
	}
	// a HUD function must not be exempt from the canvas
	src = guardHeader + "func p(dst I) { vector.FillRect(dst, 0, 0, 1, 1, c, false) }\n"
	got = guardOn(t, "hud.go", src, map[string]string{"hud.go:p": "oops"}, nil)
	if len(got) != 1 || !strings.Contains(got[0], "rule 3") {
		t.Errorf("an exempt HUD function = %q, want rule 3", got)
	}
}

func TestGuardStaleEntries(t *testing.T) {
	src := guardHeader + "func a(dst I) { vector.FillRect(dst, 0, 0, 1, 1, c, false) }\nfunc b() {}\n"
	got := guardOn(t, "x.go", src,
		map[string]string{"x.go:a": "world", "x.go:b": "gone raw", "x.go:ghost": "deleted"},
		map[string]string{"x.go:b": "gone"})
	want := []string{`stale: canvasExempt "x.go:b"`, `stale: canvasExempt "x.go:ghost"`, `stale: worldDrawsLogical "x.go:b"`}
	if len(got) != len(want) {
		t.Fatalf("violations = %q, want %q", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("violation %d = %q, want prefix %q", i, got[i], want[i])
		}
	}
}

// The name lists the guard keys on must exist in the package; the real
// package scan covers them (declared names) in TestUIDrawsThroughCanvas.
func TestGuardNameListsMatchThePackage(t *testing.T) {
	declared, _ := declaredFuncs(t, ".")
	for n := range logicalDraws {
		if !declared[n] {
			t.Errorf("logicalDraws names %q, not declared", n)
		}
	}
	for n := range worldPixelHelpers {
		if !declared[n] {
			t.Errorf("worldPixelHelpers names %q, not declared", n)
		}
	}
}
