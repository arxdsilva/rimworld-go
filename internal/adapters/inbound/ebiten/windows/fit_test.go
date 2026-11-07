package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The fake shell has no clock and six factions to show (Wildlife, Factions).
func (s *fakeShell) Tick() clock.Tick    { return 0 }
func (s *fakeShell) Factions() []Faction { return make([]Faction, 6) }

// TestMainTabWindowsFitOneStepScreen (UIS-2c): at the smallest screen a UI
// step above 1 allows (1280 x 768 logical) every main-tab window, opened on a
// small colony, is given its whole wanted width, and its height up to the room
// above the tab bar. Research and History fill the room, so they may be
// shorter than they want.
func TestMainTabWindowsFitOneStepScreen(t *testing.T) {
	t.Parallel()
	const sw, sh = 1280, 768
	open := map[string]func(*Set){
		"work":     func(w *Set) { w.Show("work") },
		"schedule": func(w *Set) { w.Show("schedule") },
		"animals":  func(w *Set) { w.ToggleAnimals() },
		"wildlife": func(w *Set) { w.ToggleWildlife() },
		"assign":   func(w *Set) { w.Assign().Launch() },
		"history":  func(w *Set) { w.History().Launch() },
		"factions": func(w *Set) { w.Factions().Launch() },
		"research": func(w *Set) { w.Show("research") },
	}
	// Factions and History may be cut: their lists scroll.
	fills := map[string]bool{"history": true, "research": true, "factions": true, "wildlife": true}
	for name, do := range open {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fs, w := newFake()
			fs.env.Game = fakeReader{}
			for i := 1; i <= 3; i++ {
				fs.pawns = append(fs.pawns, game.PawnView{ID: entity.EntityID(i), Name: "P", Status: "alive"})
			}
			do(w)
			ww, wh := w.Want()
			if ww <= 0 || wh <= 0 {
				t.Fatalf("%s wants nothing: %vx%v", name, ww, wh)
			}
			l := hudlayout.Compute(sw, sh, hudlayout.Inputs{Colonists: 3, WindowW: ww, WindowH: wh, FullWidthWindow: name == "research"}) // UIS-065
			if wild, drops := name == "wildlife", name == "animals" || name == "wildlife"; drops {
				full := colonyFullW
				if wild { // UIS-064: the Wild list is WildW wide and drops nothing
					full = hudlayout.WildW
				}
				for _, c := range animalColsFor(l.Window.W, wild) {
					full -= c.w
				}
				if float32(full) > l.Window.W { // the list drops columns to fit
					t.Errorf("%s: even without its optional columns it needs %d of %v", name, full, l.Window.W)
				}
			} else if l.Window.W < ww {
				t.Errorf("%s is %v wide on a %dx%d screen, wants %v", name, l.Window.W, sw, sh, ww)
			}
			if l.Window.Bottom() > l.Tabs.Y {
				t.Errorf("%s reaches into the tab bar: bottom %v, tabs %v", name, l.Window.Bottom(), l.Tabs.Y)
			}
			if !fills[name] && l.Window.H < wh {
				t.Errorf("%s is %v tall, wants %v", name, l.Window.H, wh)
			}
		})
	}
}
