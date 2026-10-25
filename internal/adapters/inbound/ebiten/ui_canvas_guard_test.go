package ebiten

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// rawDrawRE matches a draw that bypasses the UI canvas (ui_canvas.go).
var rawDrawRE = regexp.MustCompile(`\bvector\.|\bebitenutil\.|\.DrawImage\(|\.DrawTriangles(32)?\(|\.DrawTrianglesShader(32)?\(|\.DrawRectShader\(|\btext\.Draw\(`)

// canvasExempt are the files that may draw raw for good, in physical
// pixels: the canvas itself, the world renderer (everything the camera
// positions) and the full-screen backgrounds. A file that mixes world
// and UI drawing belongs here only once its UI parts use the canvas.
var canvasExempt = map[string]string{
	"ui_canvas.go":          "the canvas: logical in, physical out",
	"font.go":               "the canvas's text path: scaled faces at physical positions",
	"renderer.go":           "world renderer",
	"draw_art.go":           "world: terrain, objects, ghosts",
	"draw_combat.go":        "world: combat effects",
	"anim.go":               "world: sprite animation and particles",
	"pawn_anim.go":          "world: pawn figures",
	"animal_figure.go":      "world: animal figures",
	"burial_anim.go":        "world: corpses, graves, pyres",
	"arrival_draw.go":       "world: drop-pod arrivals",
	"bridge_draw.go":        "world: bridge water shadows and fallback deck",
	"cave_plants_draw.go":   "world: the duskmoss glow discs",
	"biome_art.go":          "world: biome terrain and fallbacks",
	"blood_draw.go":         "world: blood batches",
	"dirt_draw.go":          "world: dirt batches",
	"features_draw.go":      "world: steam vent puffs and ruin overlays",
	"fog_draw.go":           "world: fog of war chunks over the map",
	"mapfields_overlay.go":  "world: dev map-field layer over the map",
	"environment.go":        "world: weather, fog and veils over the map",
	"fence.go":              "world: fences and alerts on tiles",
	"formation.go":          "world: formation markers on tiles",
	"grass_batch.go":        "world: grass batches",
	"light.go":              "world: the light pass",
	"lod.go":                "world: chunk LOD bakes",
	"paths_draw.go":         "world: path lines",
	"shadow.go":             "world: shadows",
	"shadow_cast.go":        "world: cast shadows",
	"sprites.go":            "world sprite blit (UI callers use uiImage)",
	"building_inspector.go": "mixed: the selection outline is world (UI parts use the canvas)",
	"room_stats.go":         "mixed: the room tile tint is world (the hover card uses the canvas)",
	"labels.go":             "world: overlays anchored to tiles (stack-count text goes through the canvas)",
	"multiselect.go":        "world: group marks on tiles (panel and drag box use the canvas)",
	"order_queue_ui.go":     "world: order route over the map (text goes through the canvas)",
	"world_ui.go":           "globe overlay: caravan marks drawn on the globe surface",
	"water_draw.go":         "world: water",
	"zones_draw.go":         "world: zones",
	"architect.go":          "world: zone/drag ghost cells on the map (palette and labels use the canvas)",
	"move_mode.go":          "world: move-mode footprint outline (hints use the canvas)",
	"menu_diorama.go":       "full-screen background (physical size, unscaled)",
	"setup_backdrop.go":     "full-screen background (physical size, unscaled)",
	"globe.go":              "globe: drawn at physical size inside its logical area (x s); names go through the canvas",
}

// pendingCanvasSweep is TEMPORARY (PLAY-012 B.4): the UI files that
// still draw raw. Each sweep agent converts its files to the canvas and
// deletes them here; a mixed world/UI file moves to canvasExempt with a
// reason once its UI parts are converted. This list must end empty.
var pendingCanvasSweep = map[string]bool{}

// rawDrawLines are the lines of src (comments stripped) that draw raw.
func rawDrawLines(src string) []string {
	var out []string
	for i, line := range strings.Split(src, "\n") {
		if j := strings.Index(line, "//"); j >= 0 {
			line = line[:j]
		}
		if rawDrawRE.MatchString(line) {
			out = append(out, strings.TrimSpace(line)+"  (line "+strconv.Itoa(i+1)+")")
		}
	}
	return out
}

// TestUIDrawsThroughCanvas (PLAY-012 B.4): UI is laid out in logical
// pixels and drawn through the canvas helpers, so it scales. Only the
// listed files may call vector, ebitenutil, DrawImage, DrawTriangles or
// text.Draw directly. Entries for files that no longer draw raw fail too,
// so the lists stay honest.
func TestUIDrawsThroughCanvas(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var bad []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		raw := rawDrawLines(string(src))
		if len(raw) == 0 {
			continue
		}
		seen[f] = true
		if canvasExempt[f] != "" || pendingCanvasSweep[f] {
			continue
		}
		bad = append(bad, f+":\n\t"+strings.Join(raw, "\n\t"))
	}
	if len(bad) > 0 {
		t.Errorf("UI files must draw through ui_canvas.go (uiFillRect, uiLine, uiImage, drawText, ...):\n%s", strings.Join(bad, "\n"))
	}
	var stale []string
	for f := range canvasExempt {
		if !seen[f] {
			stale = append(stale, f+" (canvasExempt)")
		}
	}
	for f := range pendingCanvasSweep {
		if canvasExempt[f] != "" {
			stale = append(stale, f+" (in both lists)")
		} else if !seen[f] {
			stale = append(stale, f+" (pendingCanvasSweep: converted, delete the entry)")
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("stale canvas allowlist entries (the file no longer draws raw):\n\t%s", strings.Join(stale, "\n\t"))
	}
}

func TestRawDrawLines(t *testing.T) {
	src := "a := 1\nvector.FillRect(dst, 0, 0, 1, 1, c, false) // x\n// vector.FillRect in a comment\nuiFillRect(dst, 0, 0, 1, 1, c, false)\nscreen.DrawImage(img, op)\n"
	if got := rawDrawLines(src); len(got) != 2 || !strings.Contains(got[0], "line 2") || !strings.Contains(got[1], "line 5") {
		t.Fatalf("rawDrawLines = %q", got)
	}
}
