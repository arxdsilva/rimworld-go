package ebiten

import (
	"fmt"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-017 §11: an animal chasing a colonist is shown, not just announced.

// --- danger messages are red ------------------------------------------

func TestDangerMessageIsRed(t *testing.T) {
	t.Parallel()
	danger := messageStyle(game.MessageView{Text: "A timber wolf is chasing Ada!", Danger: true, Focus: 4})
	if danger.Color != uiDanger {
		t.Fatalf("danger message colour = %v, want the UI danger red %v", danger.Color, uiDanger)
	}
	r, g, b, _ := danger.Color.RGBA()
	if r <= g || r <= b {
		t.Fatalf("danger colour %v is not red", danger.Color)
	}
	calm := messageStyle(game.MessageView{Text: "A deer gave birth", Focus: 4})
	if calm.Color != nil {
		t.Fatalf("an ordinary message keeps the default colour, got %v", calm.Color)
	}
	if danger.Size != tsBody.Size || calm.Size != tsBody.Size {
		t.Fatalf("message text stays body size: %v / %v", danger.Size, calm.Size)
	}
}

// A red message is still a button that jumps to its animal, and drawing
// the list does not panic.
func TestDangerMessageStaysClickable(t *testing.T) {
	t.Parallel()
	sim := &worldSim{stubSim: &stubSim{w: 40, h: 40}, w: game.WorldView{
		Animals:  []game.AnimalView{{ID: entity.EntityID(5), Species: "timber_wolf", Label: "timber wolf", Hostile: true, TargetID: 1, X: 30, Y: 31}},
		Messages: []game.MessageView{{Tick: 1, Text: "A timber wolf is chasing Ada!", Danger: true, Focus: 5}},
	}}
	a := appWith(sim, nil, nil)
	a.finishBoot()
	a.startGame()
	a.sw, a.sh = 1280, 720
	found := false
	for _, b := range a.messageButtons(a.sw) {
		found = found || b.id == "msg.0"
	}
	if !found {
		t.Fatal("a danger message with a Focus is clickable")
	}
	a.drawMessages(ebiten.NewImage(1280, 720))
	a.applyClick("msg.0")
	if !a.hasAnimal || a.selAnimal != 5 {
		t.Fatalf("selected animal = %v/%d, want 5", a.hasAnimal, a.selAnimal)
	}
}

// --- the marker and the chase line ------------------------------------

// markTrace is one world-pass mark animal overlays drew.
type markTrace struct {
	kind       string
	a, b, c, d float32
}

// traceMarks records the marks r's drawAnimalOverlays draws in the world pass.
func traceMarks(t *testing.T, r *Renderer) *[]markTrace {
	t.Helper()
	var got []markTrace
	r.view.Trace = &render.Trace{AnimalMark: func(kind string, a, b, c, d float32) {
		got = append(got, markTrace{kind, a, b, c, d})
	}}
	t.Cleanup(func() { r.view.Trace = nil })
	return &got
}

func ofKind(marks []markTrace, kind string) []markTrace {
	var out []markTrace
	for _, m := range marks {
		if m.kind == kind {
			out = append(out, m)
		}
	}
	return out
}

func near32(a, b float32) bool { return math.Abs(float64(a-b)) < 0.51 }

// chaseWorld: Ada (colonist 1) at (6,5) is chased by wolf 10 at (9,5) and
// not by boar 11 (hostile, no target), a manhunting deer 12 (enraged, no
// target), or a peaceful cow 13 and a cow 14 that is only marked for
// taming.
func chaseWorld() (game.WorldView, []game.PawnView) {
	pawns := []game.PawnView{{ID: 1, Name: "Ada", X: 6, Y: 5}}
	w := game.WorldView{Animals: []game.AnimalView{
		{ID: 10, Species: "timber_wolf", Label: "timber wolf", Size: 0.9, Health: 100, X: 9, Y: 5, Hostile: true, TargetID: 1},
		{ID: 11, Species: "boar", Label: "boar", Size: 0.8, Health: 100, X: 3, Y: 8, Hostile: true},
		{ID: 12, Species: "deer", Label: "deer", Size: 0.9, Health: 100, X: 12, Y: 8, Hostile: true, Manhunter: true},
		{ID: 13, Species: "cow", Label: "cow", Size: 1.2, Health: 100, X: 3, Y: 3},
		{ID: 14, Species: "cow", Label: "cow", Size: 1.2, Health: 100, X: 5, Y: 11, TameMark: true},
	}}
	return w, pawns
}

func TestHostileAnimalsGetAMarkAndOnlyChasersGetALine(t *testing.T) {
	r := New(&stubSim{w: 32, h: 32})
	w, pawns := chaseWorld()
	r.view.Pawns, r.view.W = pawns, w
	marks := traceMarks(t, r)
	r.drawAnimalOverlays(ebiten.NewImage(1024, 768), w, 64, 1024, 768)

	// a red marker on every hostile animal (wolf, boar, manhunting deer)
	// and on nothing else
	got := ofKind(*marks, "hostile-marker")
	want := map[entity.EntityID]bool{10: true, 11: true, 12: true}
	if len(got) != len(want) {
		t.Fatalf("%d hostile markers drawn (%+v), want %d", len(got), got, len(want))
	}
	for _, an := range w.Animals {
		sx, sy := r.worldToScreenF(float64(an.X), float64(an.Y))
		hit := false
		for _, m := range got {
			hit = hit || (near32(m.a, sx) && near32(m.b, sy))
		}
		if hit != want[an.ID] {
			t.Errorf("animal %d (%s): hostile marker drawn = %v, want %v", an.ID, an.Label, hit, want[an.ID])
		}
	}

	// a chase line only for the wolf, from its centre to Ada's, in
	// physical pixels
	lines := ofKind(*marks, "chase-line")
	if len(lines) != 1 {
		t.Fatalf("%d chase lines (%+v), want 1", len(lines), lines)
	}
	x0, y0 := r.worldToScreenF(9.5, 5.5)
	x1, y1 := r.worldToScreenF(6.5, 5.5)
	if l := lines[0]; !near32(l.a, x0) || !near32(l.b, y0) || !near32(l.c, x1) || !near32(l.d, y1) {
		t.Fatalf("chase line %+v, want (%v,%v) -> (%v,%v)", l, x0, y0, x1, y1)
	}
}

func TestChaseLineEndsAtTheDrawnPositions(t *testing.T) {
	r := New(&stubSim{w: 32, h: 32})
	w, pawns := chaseWorld()
	r.view.Pawns, r.view.W = pawns, w
	// both creatures are drawn off the tile the read model reports (as
	// mid-step): the line follows what is drawn
	r.mo.tracks = map[entity.EntityID]*moTrack{
		10: {x: 8, y: 5},
		1:  {x: 6, y: 6},
	}
	marks := traceMarks(t, r)
	r.drawAnimalOverlays(ebiten.NewImage(1024, 768), w, 64, 1024, 768)
	lines := ofKind(*marks, "chase-line")
	if len(lines) != 1 {
		t.Fatalf("chase lines %+v", lines)
	}
	x0, y0 := r.worldToScreenF(8.5, 5.5)
	x1, y1 := r.worldToScreenF(6.5, 6.5)
	if l := lines[0]; !near32(l.a, x0) || !near32(l.b, y0) || !near32(l.c, x1) || !near32(l.d, y1) {
		t.Fatalf("chase line %+v, want the interpolated (%v,%v) -> (%v,%v)", l, x0, y0, x1, y1)
	}
}

func TestChaseLineSkippedOffScreenAndWithoutATarget(t *testing.T) {
	r := New(&stubSim{w: 64, h: 64})
	w, pawns := chaseWorld()
	r.view.W = w
	marks := traceMarks(t, r)
	img := ebiten.NewImage(1024, 768)

	// the target is not a pawn we can see: no line
	r.view.Pawns = nil
	r.drawAnimalOverlays(img, w, 64, 1024, 768)
	if n := len(ofKind(*marks, "chase-line")); n != 0 {
		t.Fatalf("%d lines to a missing target", n)
	}

	// both ends far outside the view: culled
	r.view.Pawns = pawns
	r.lod.viewSet = true
	r.lod.vx0, r.lod.vy0, r.lod.vx1, r.lod.vy1 = 40, 40, 60, 60
	*marks = nil
	r.drawAnimalOverlays(img, w, 64, 1024, 768)
	if n := len(ofKind(*marks, "chase-line")); n != 0 {
		t.Fatalf("%d lines drawn off screen", n)
	}

	// the colonist is in view though the wolf is not: the line is drawn
	r.lod.vx0, r.lod.vy0, r.lod.vx1, r.lod.vy1 = 0, 0, 7, 10
	w.Animals[0].X = 16
	r.view.W = w
	*marks = nil
	r.drawAnimalOverlays(img, w, 64, 1024, 768)
	if n := len(ofKind(*marks, "chase-line")); n != 1 {
		t.Fatalf("%d lines with the colonist in view, want 1", n)
	}
}

// --- tame names ---------------------------------------------------------

func TestTameNameSitsUnderItsAnimalAtScale(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprint("scale ", s), func(t *testing.T) {
			withUIScale(t, s)
			r := New(&stubSim{w: 32, h: 32})
			an := game.AnimalView{ID: 3, Species: "cow", Label: "cow", Tame: true, Name: "Bessie", Size: 1.2, Health: 100, X: 5, Y: 6}
			w := game.WorldView{Animals: []game.AnimalView{an}}
			r.view.W = w
			marks := traceMarks(t, r)
			draw := func() ([]markTrace, []markTrace) {
				*marks = nil
				r.drawAnimalOverlays(ebiten.NewImage(1024, 768), w, 64, 1024, 768)
				return ofKind(*marks, "name-plate"), ofKind(*marks, "name-text")
			}
			plates, texts := draw()
			if len(plates) != 1 || len(texts) != 1 {
				t.Fatalf("name drawn as %d plates, %d texts", len(plates), len(texts))
			}
			p, tx := plates[0], texts[0]

			// under the animal: the plate hangs from the animal's feet
			// (the bottom of its tile) and is centred on it, all physical
			ax, ay := r.worldToScreenF(5.5, 7.0)
			if !near32(p.a+p.c/2, ax) || !near32(p.b, ay) {
				t.Errorf("plate %+v is not centred under the animal at (%v,%v)", p, ax, ay)
			}
			// sized in the same units as the text: logical width + 6 and
			// 14 high, times the UI scale
			us := float32(s)
			if wantW := (textW("Bessie", tsSmall) + 6) * us; !near32(p.c, wantW) || !near32(p.d, 14*us) {
				t.Errorf("plate is %vx%v, want %vx%v", p.c, p.d, wantW, 14*us)
			}
			// the text anchor is the plate's centre top (physical, drawn
			// once through worldText), not scaled twice
			if !near32(tx.a, p.a+p.c/2) || tx.b < p.b-3*us || tx.b > p.b+p.d {
				t.Errorf("name anchored at (%v,%v), plate %+v", tx.a, tx.b, p)
			}

			// pan: every world mark moves by exactly -delta physical
			r.cam.offsetX += 40
			r.cam.offsetY -= 24
			p2s, t2s := draw()
			if len(p2s) != 1 || len(t2s) != 1 {
				t.Fatal("name not drawn after the pan")
			}
			if !near32(p2s[0].a, p.a-40) || !near32(p2s[0].b, p.b+24) || !near32(p2s[0].c, p.c) {
				t.Errorf("plate after pan %+v, before %+v", p2s[0], p)
			}
			if !near32(t2s[0].a, tx.a-40) || !near32(t2s[0].b, tx.b+24) {
				t.Errorf("text after pan %+v, before %+v", t2s[0], tx)
			}
		})
	}
}

func TestTameNameHiddenWhenZoomedOutOrUnnamed(t *testing.T) {
	r := New(&stubSim{w: 32, h: 32})
	w := game.WorldView{Animals: []game.AnimalView{
		{ID: 3, Species: "cow", Tame: true, Name: "Bessie", Size: 1.2, Health: 100, X: 5, Y: 6},
		{ID: 4, Species: "cow", Tame: true, Size: 1.2, Health: 100, X: 7, Y: 6},
		{ID: 5, Species: "cow", Name: "Wild", Size: 1.2, Health: 100, X: 9, Y: 6},
	}}
	marks := traceMarks(t, r)
	r.drawAnimalOverlays(ebiten.NewImage(1024, 768), w, 64, 1024, 768)
	if n := len(ofKind(*marks, "name-plate")); n != 1 {
		t.Fatalf("%d name plates, want only the named tame animal's", n)
	}
	*marks = nil
	r.drawAnimalOverlays(ebiten.NewImage(1024, 768), w, 8, 1024, 768)
	if n := len(ofKind(*marks, "name-plate")); n != 0 {
		t.Fatalf("%d name plates when zoomed far out", n)
	}
}

// --- the camera on a chase auto-pause -------------------------------------

// chaseCamera is a renderer on a 64x64 map at zoom 1 (64 px tiles), top
// left of the map at the top left of an 800x520 viewport, with colonist 1
// at (cx, cy) chased by animal 10 at (ax, ay).
func chaseCamera(cx, cy, ax, ay int) (*Renderer, game.WorldView) {
	r := New(&stubSim{w: 64, h: 64, pawns: []game.PawnView{{ID: 1, Name: "Ada", X: cx, Y: cy}}})
	r.cam = camera{zoom: 1, placed: true}
	r.voice.view = viewport{0, 40, 800, 560}
	w := game.WorldView{
		Animals: []game.AnimalView{{ID: 10, Species: "timber_wolf", Label: "timber wolf", Hostile: true, TargetID: 1, X: ax, Y: ay}},
		Messages: []game.MessageView{
			{Tick: 5, Text: "A timber wolf is chasing Ada!", Danger: true, Focus: 10},
		},
	}
	return r, w
}

func TestThreatPauseCentresAnOffScreenChase(t *testing.T) {
	t.Parallel()
	// wolf at (50,50), Ada at (52,50): far right and below the view
	r, w := chaseCamera(52, 50, 50, 50)
	r.onThreatPause(w, 5)
	// the midpoint of the two tile centres is mid-viewport
	sx, sy := r.worldToScreenF(51.5, 50.5)
	if math.Abs(float64(sx)-400) > 1 || math.Abs(float64(sy)-300) > 1 {
		t.Fatalf("midpoint of the chase at (%v,%v), want the viewport centre (400,300)", sx, sy)
	}
}

func TestThreatPauseLeavesAnOnScreenChaseAlone(t *testing.T) {
	t.Parallel()
	r, w := chaseCamera(6, 5, 7, 5)
	before := r.cam
	r.onThreatPause(w, 5)
	if r.cam != before {
		t.Fatalf("camera moved for a chase already in view: %+v -> %+v", before, r.cam)
	}
}

func TestThreatPauseCentresWhenOnlyOneEndIsOffScreen(t *testing.T) {
	t.Parallel()
	// Ada in view, the wolf bounding in from the right of it
	r, w := chaseCamera(10, 5, 13, 5)
	before := r.cam
	r.onThreatPause(w, 5)
	if r.cam == before {
		t.Fatal("the wolf is off screen, so the camera must show the chase")
	}
	sx, _ := r.worldToScreenF(12, 5.5)
	if math.Abs(float64(sx)-400) > 1 {
		t.Fatalf("midpoint at x %v, want 400", sx)
	}
}

func TestThreatPauseIgnoresOtherDangers(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		desc string
		edit func(w *game.WorldView)
	}{
		{"a raid with no creature to focus", func(w *game.WorldView) {
			w.Messages = []game.MessageView{{Tick: 5, Text: "Raiders!", Danger: true, X: 50, Y: 50}}
		}},
		{"a danger focused on an animal that is not chasing", func(w *game.WorldView) {
			w.Animals[0].TargetID = 0
		}},
		{"a focused animal the world no longer has", func(w *game.WorldView) {
			w.Messages[0].Focus = 99
		}},
		{"a chase message that is not a danger", func(w *game.WorldView) {
			w.Messages[0].Danger = false
		}},
		{"an old chase when the newest danger is a raid", func(w *game.WorldView) {
			w.Messages = append(w.Messages, game.MessageView{Tick: 9, Text: "Raiders!", Danger: true})
		}},
		{"a chase from an earlier tick", func(w *game.WorldView) {
			w.Messages[0].Tick = 4
		}},
		{"a target who is gone", func(w *game.WorldView) {
			w.Animals[0].TargetID = 77
		}},
	} {
		r, w := chaseCamera(52, 50, 50, 50)
		c.edit(&w)
		tick := uint64(5)
		if c.desc == "an old chase when the newest danger is a raid" {
			tick = 9
		}
		before := r.cam
		r.onThreatPause(w, tick)
		if r.cam != before {
			t.Errorf("%s: camera moved %+v -> %+v", c.desc, before, r.cam)
		}
	}
}

func TestThreatPauseWithoutAViewportKeepsTheCamera(t *testing.T) {
	t.Parallel()
	r, w := chaseCamera(52, 50, 50, 50)
	r.voice.view = viewport{} // no App has set the viewport yet
	before := r.cam
	r.onThreatPause(w, 5)
	if r.cam != before {
		t.Fatalf("camera moved with no viewport: %+v", r.cam)
	}
}

// The real path: a new chase warning pauses the game (the existing
// auto-pause) and brings an off-screen chase into view; an on-screen one
// pauses without moving the camera.
func TestWatchThreatsShowsAnOffScreenChase(t *testing.T) {
	t.Parallel()
	r, w := chaseCamera(52, 50, 50, 50)
	quiet := w
	quiet.Messages = nil
	r.watchThreats(quiet) // the session starts: nothing pauses
	before := r.cam
	r.watchThreats(w)
	if r.speed != speedPaused || r.speedRate() != "paused (threat)" {
		t.Fatalf("a new chase pauses: %v %q", r.speed, r.speedRate())
	}
	if r.cam == before {
		t.Fatal("the pause shows the off-screen chase")
	}

	r, w = chaseCamera(6, 5, 7, 5)
	r.watchThreats(quiet)
	before = r.cam
	r.watchThreats(w)
	if r.speed != speedPaused || r.cam != before {
		t.Fatalf("an on-screen chase pauses without moving the camera: %v %+v -> %+v", r.speed, before, r.cam)
	}

	// option off: no pause, no camera move
	r, w = chaseCamera(52, 50, 50, 50)
	r.pauseOnThreat = false
	r.watchThreats(quiet)
	before = r.cam
	r.watchThreats(w)
	if r.speed == speedPaused || r.cam != before {
		t.Fatalf("pause on threat off: %v, camera %+v -> %+v", r.speed, before, r.cam)
	}
}
