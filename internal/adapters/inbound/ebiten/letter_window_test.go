package ebiten

import (
	"fmt"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"
)

// An opened letter (UIS-073) is frameless: no title bar and no X. Its
// title is drawn in the body in the Title face at the top, and its
// choices are plain rows MenuRowH apart, in reading order, ending
// WindowPad above the frame's bottom. Not parallel: the draw trace is
// process-wide.
func TestFramelessLetterHasNoX(t *testing.T) {
	s := letterStub(letter(21, "info", "Quiet day"))
	s.world.Letters[0].X, s.world.Letters[0].Y = 30, 31
	a, f := colonyApp(t, s)
	pressButton(t, a, f, "letter.21")
	f.Frame(t, a)
	if top := a.uiModalTop(); top != "letter.21" {
		t.Fatalf("top modal %q", top)
	}
	m := a.uiSt().Modals[0]
	if !m.Frameless || !m.RowActions {
		t.Fatalf("letter modal frameless %v rows %v", m.Frameless, m.RowActions)
	}
	if hasWidget(a, "modal.letter.21.close") {
		t.Fatal("a frameless letter has an X")
	}
	frame, _, _, _ := uikit.ModalGeometry(a.sw, a.sh, m)
	if frame.W != 620 || frame.H != 480 {
		t.Fatalf("letter frame %+v, want 620 x 480", frame)
	}
	cont, ok1 := a.uiSt().Find("modal.letter.21.continue")
	jump, ok2 := a.uiSt().Find("modal.letter.21.jump")
	if !ok1 || !ok2 {
		t.Fatal("the letter's rows are not declared")
	}
	if cont.R.H != hudlayout.MenuRowH || jump.R.Y-cont.R.Y != hudlayout.MenuRowH || jump.R.Bottom() != frame.Bottom()-hudlayout.WindowPad ||
		cont.R.X != frame.X+hudlayout.WindowPad || cont.R.Right() != frame.Right()-hudlayout.WindowPad {
		t.Fatalf("rows continue %+v jump %+v in frame %+v", cont.R, jump.R, frame)
	}

	var evs []gfx.TraceEvent
	gfx.SetDrawTrace(func(e gfx.TraceEvent) { evs = append(evs, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	a.uiSt().Paint(ebiten.NewImage(a.sw, a.sh))
	gfx.SetDrawTrace(nil)
	sc := float32(a.scale.S)
	titles := 0
	for _, e := range evs {
		if e.Kind != "text" || e.Text != "Quiet day" {
			continue
		}
		titles++
		x, y, w := e.X/sc, e.Y/sc, e.W/sc
		if e.Size != gfx.TitleSize*sc {
			t.Errorf("the title is drawn at %v px, want the Title face", e.Size/sc)
		}
		if x < frame.X+hudlayout.WindowPad-0.6 || x+w > frame.Right()-hudlayout.WindowPad+0.6 || y < frame.Y+hudlayout.WindowPad-0.6 || y > frame.Y+hudlayout.WindowPad+0.6 {
			t.Errorf("the title at %v,%v (w %v) is not at the top of the body of %+v", x, y, w, frame)
		}
	}
	if titles != 1 {
		t.Fatalf("the title is drawn %d times, want once in the body", titles)
	}
}

// TestJourneyLetterEscCloses: a letter opened from the stack closes with
// Escape, which neither dismisses the letter nor opens the pause menu.
func TestJourneyLetterEscCloses(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.FireIncident("flu") || len(j.world().Letters) == 0 {
		t.Fatal("the flu outbreak posted no letter")
	}
	j.update(2)
	id := j.world().Letters[len(j.world().Letters)-1].ID
	if !j.clickID(fmt.Sprintf("letter.%d", id)) || j.a.uiModalTop() != fmt.Sprintf("letter.%d", id) {
		t.Fatalf("the letter row opened %q", j.a.uiModalTop())
	}
	j.update(1)
	if _, ok := j.a.uiSt().Find(fmt.Sprintf("modal.letter.%d.close", id)); ok {
		t.Fatal("the letter has an X")
	}
	j.key("Escape", 0)
	if j.a.uiSt().ModalOpen() {
		t.Fatalf("Escape left %q open", j.a.uiModalTop())
	}
	j.update(1)
	if j.a.nav.screen() != ports.ScreenColony || j.a.nav.overlay() != overlayNone {
		t.Fatalf("Escape on the letter went on to screen %s overlay %v", j.a.nav.screen(), j.a.nav.overlay())
	}
	for _, c := range j.cmds {
		if c.Kind == command.DismissLetter {
			t.Fatalf("Escape dismissed the letter: %+v", c)
		}
	}
	if len(j.world().Letters) == 0 || j.world().Letters[len(j.world().Letters)-1].ID != id {
		t.Fatal("the letter left the stack")
	}
}
