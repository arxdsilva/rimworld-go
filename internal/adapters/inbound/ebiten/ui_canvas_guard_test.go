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
	"ui_canvas.go":      "the canvas: logical in, physical out",
	"font.go":           "the canvas's text path: scaled faces at physical positions",
	"renderer.go":       "world renderer",
	"draw_art.go":       "world: terrain, objects, ghosts",
	"draw_combat.go":    "world: combat effects",
	"anim.go":           "world: sprite animation and particles",
	"pawn_anim.go":      "world: pawn figures",
	"animal_figure.go":  "world: animal figures",
	"burial_anim.go":    "world: corpses, graves, pyres",
	"arrival_draw.go":   "world: drop-pod arrivals",
	"biome_art.go":      "world: biome terrain and fallbacks",
	"blood_draw.go":     "world: blood batches",
	"dirt_draw.go":      "world: dirt batches",
	"environment.go":    "world: weather, fog and veils over the map",
	"fence.go":          "world: fences and alerts on tiles",
	"formation.go":      "world: formation markers on tiles",
	"grass_batch.go":    "world: grass batches",
	"light.go":          "world: the light pass",
	"lod.go":            "world: chunk LOD bakes",
	"paths_draw.go":     "world: path lines",
	"shadow.go":         "world: shadows",
	"shadow_cast.go":    "world: cast shadows",
	"sprites.go":        "world sprite blit (UI callers use uiImage)",
	"water_draw.go":     "world: water",
	"zones_draw.go":     "world: zones",
	"menu_diorama.go":   "full-screen background (physical size, unscaled)",
	"setup_backdrop.go": "full-screen background (physical size, unscaled)",
}

// pendingCanvasSweep is TEMPORARY (PLAY-012 B.4): the UI files that
// still draw raw. Each sweep agent converts its files to the canvas and
// deletes them here; a mixed world/UI file moves to canvasExempt with a
// reason once its UI parts are converted. This list must end empty.
var pendingCanvasSweep = map[string]bool{
	"alerts.go":             true,
	"animal_inspector.go":   true,
	"animals_ui.go":         true,
	"architect.go":          true,
	"building_inspector.go": true,
	"climate_ui.go":         true,
	"faction_rows.go":       true,
	"factions_window.go":    true,
	"frame_inspector.go":    true,
	"gathering_ui.go":       true,
	"gizmo.go":              true,
	"globe.go":              true,
	"hud.go":                true,
	"inspector.go":          true,
	"labels.go":             true,
	"minimap.go":            true,
	"move_mode.go":          true,
	"multiselect.go":        true,
	"order_queue_ui.go":     true,
	"orders.go":             true,
	"portrait.go":           true,
	"research_ui.go":        true,
	"room_stats.go":         true,
	"schedule_ui.go":        true,
	"screen_options.go":     true,
	"screen_scenario.go":    true,
	"screen_storyteller.go": true,
	"search_ui.go":          true,
	"status_chips.go":       true,
	"story_tab.go":          true,
	"threat_ui.go":          true,
	"trough_ui.go":          true,
	"work.go":               true,
	"world_ui.go":           true,
}

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
