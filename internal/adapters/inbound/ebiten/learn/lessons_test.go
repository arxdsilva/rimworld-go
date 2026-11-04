package learn

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

func TestLessonCatalog(t *testing.T) {
	t.Parallel()
	want := []string{"camera", "select", "build", "zones", "work", "schedule", "draft", "forbid", "research", "speed",
		"dossier_hover", "dossier_gear", "dossier_look", "dossier_sections", "dossier_close"}
	var got []string
	seen := map[string]bool{}
	keys := newFake().KeyName
	for _, l := range Catalog {
		got = append(got, l.ID)
		if seen[l.ID] {
			t.Fatalf("duplicate lesson id %q", l.ID)
		}
		seen[l.ID] = true
		if l.OnCmd == nil && l.OnUI == nil {
			t.Errorf("%s: no way to be done on its own", l.ID)
		}
		body := l.Body(keys)
		// 2-3 short sentences, our own words
		n := strings.Count(body, ". ") + strings.Count(body, "? ") + 1
		if n < 2 || n > 3 || !strings.HasSuffix(body, ".") {
			t.Errorf("%s: body has %d sentences, want 2-3: %q", l.ID, n, body)
		}
		if len(body) > 330 {
			t.Errorf("%s: body is %d chars, keep it short", l.ID, len(body))
		}
		for _, s := range []string{l.Title, l.Goal, body} {
			if strings.Contains(strings.ToLower(s), "rimworld") || strings.Contains(s, "{") || s == "" {
				t.Errorf("%s: bad text %q", l.ID, s)
			}
		}
		if w, _ := gfx.Measure(l.Title, gfx.Label); w > RowW-RowPad*2 {
			t.Errorf("%s: title %q is %.0f px, the row holds %d", l.ID, l.Title, w, RowW-RowPad*2)
		}
		if w, _ := gfx.Measure(l.Goal, gfx.Small); w > RowW-RowPad*2 {
			t.Errorf("%s: goal %q is %.0f px, the row holds %d", l.ID, l.Goal, w, RowW-RowPad*2)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lessons = %v, want %v", got, want)
	}
}

func TestLessonTextShowsLiveKeysAndSurvivesNoBinding(t *testing.T) {
	t.Parallel()
	draft := ByID("draft")
	live := func(act input.Action) string { return "Zed" }
	if body := draft.Body(live); !strings.Contains(body, "press Zed") {
		t.Fatalf("draft lesson should name the Draft key, got %q", body)
	}
	if ByID("nosuch") != nil {
		t.Fatal("unknown lesson id found")
	}
}

func TestDraftMemoNeedsTheDraftFirst(t *testing.T) {
	t.Parallel()
	l := ByID("draft")
	var m Memo
	move := command.Command{Kind: command.MovePawn, Pawn: 1}
	if l.OnCmd(&m, move) {
		t.Fatal("a move before the draft finished the lesson")
	}
	l.OnCmd(&m, command.Command{Kind: command.DraftPawn, Pawn: 1, Enabled: true})
	if !l.OnCmd(&m, move) {
		t.Fatal("draft then move should finish the lesson")
	}
}

func TestHelperCompletesAndHooksOnce(t *testing.T) {
	t.Parallel()
	f := newFake()
	h := New(f)
	h.UI(0, 0)
	h.UI(0, 0)
	if f.hooked != 1 {
		t.Fatalf("observer installed %d times, want once", f.hooked)
	}
	h.Observe(command.Command{Kind: command.DesignateBuild, Build: 1})
	if !slices.Contains(f.s.Gameplay.LearnedLessons, "build") {
		t.Fatalf("learned %v, want build", f.s.Gameplay.LearnedLessons)
	}
	f.scripts = true
	h.Observe(command.Command{Kind: command.SetResearch})
	if slices.Contains(f.s.Gameplay.LearnedLessons, "research") {
		t.Fatal("a scripted capture is not the player")
	}
}

func TestCardLessonsNeedTheCard(t *testing.T) {
	t.Parallel()
	f := newFake()
	h := New(f)
	on := h.count()
	f.s.HUD.Enabled = false
	off := h.count()
	if on-off != 5 || h.total() != len(Catalog)-5 {
		t.Fatalf("with the card off %d lessons left of %d, total %d: want five fewer", off, on, h.total())
	}
	for _, l := range h.left() {
		if strings.HasPrefix(l.ID, "dossier_") {
			t.Fatalf("%s offered with the card off", l.ID)
		}
	}
}

// doneAfter feeds the helper one reading per Update and says which card lessons
// finished.
func doneAfter(f *fakeShell, h *Helper, readings ...Sensed) []string {
	f.sensed = Sensed{}
	h.watch() // the baseline
	for _, s := range readings {
		f.sensed = s
		h.watch()
	}
	var out []string
	for _, id := range f.s.Gameplay.LearnedLessons {
		if strings.HasPrefix(id, "dossier_") {
			out = append(out, id)
		}
	}
	return out
}

func TestCardLessonsFinishOnWhatThePlayerDoes(t *testing.T) {
	t.Parallel()
	repeat := func(s Sensed, n int) []Sensed {
		out := make([]Sensed, n)
		for i := range out {
			out[i] = s
		}
		return out
	}
	on := func(s Sensed) Sensed { s.SetupOpen = true; return s }
	for _, c := range []struct {
		name     string
		readings []Sensed
		want     []string
	}{
		{"a pass over the card teaches nothing", append(repeat(Sensed{CardHover: true}, CardRestFrames-1), Sensed{}), nil},
		{"resting on it does", repeat(Sensed{CardHover: true}, CardRestFrames), []string{"dossier_hover"}},
		{"breaking the rest starts it over", append(append(repeat(Sensed{CardHover: true}, CardRestFrames-1), Sensed{}), repeat(Sensed{CardHover: true}, CardRestFrames-1)...), nil},
		{"opening the window is the gear", []Sensed{on(Sensed{})}, []string{"dossier_gear"}},
		{"a glance at Look is not reading it", repeat(on(Sensed{SetupLook: true}), LookRestFrames-1), []string{"dossier_gear"}},
		{"staying on Look is", repeat(on(Sensed{SetupLook: true}), LookRestFrames), []string{"dossier_gear", "dossier_look"}},
		{"the layout tabs", []Sensed{on(Sensed{SetupLayout: true})}, []string{"dossier_gear", "dossier_sections"}},
		{"closing", []Sensed{on(Sensed{}), {}}, []string{"dossier_gear", "dossier_close"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			if got := doneAfter(f, New(f), c.readings...); !slices.Equal(got, c.want) {
				t.Fatalf("finished %v, want %v", got, c.want)
			}
		})
	}
}

func TestWindowAlreadyUpAtTheBaselineIsNotAnOpening(t *testing.T) {
	t.Parallel()
	f := newFake()
	h := New(f)
	f.sensed = Sensed{SetupOpen: true}
	h.watch()
	h.watch()
	if len(f.s.Gameplay.LearnedLessons) != 0 {
		t.Fatalf("learned %v from a window that was already up", f.s.Gameplay.LearnedLessons)
	}
}
