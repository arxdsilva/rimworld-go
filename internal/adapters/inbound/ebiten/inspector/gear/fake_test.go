package gear

import (
	"fmt"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"
)

// fake is the tab's Host and Facts for unit tests: no art, a recorded
// command and intent list, a fixed screen for the kit.
type fake struct {
	kit       *uikit.Kit
	frame     inputframe.Frame
	cmds      []command.Command
	intents   []ports.Intent
	shift     bool
	unloading bool
}

type kitHost struct{ f *fake }

func (h kitHost) Frame() *inputframe.Frame { return &h.f.frame }
func (h kitHost) Cursor() (int, int)       { return -1, -1 }
func (h kitHost) Size() (int, int)         { return 1280, 720 }
func (h kitHost) Updates() uint64          { return 0 }
func (h kitHost) KeyRepeat(string) bool    { return false }
func (h kitHost) Cue(string)               {}
func (h kitHost) Hint(string)              {}
func (h kitHost) ReducedMotion() bool      { return true }

func newFake() *fake {
	f := &fake{}
	f.kit = uikit.New(kitHost{f})
	return f
}

func (f *fake) Enqueue(c command.Command)      { f.cmds = append(f.cmds, c) }
func (f *fake) Do(it ports.Intent)             { f.intents = append(f.intents, it) }
func (f *fake) Kit() *uikit.Kit                { return f.kit }
func (f *fake) Image(string) *ebiten.Image     { return nil }
func (f *fake) ItemImage(string) *ebiten.Image { return nil }
func (f *fake) Shift() bool                    { return f.shift }
func (f *fake) Unloading() bool                { return f.unloading }

func (f *fake) Temp(tenths int) string { return fmt.Sprintf("%dC", tenths/10) }

// LayerOrder puts the garments in name order (the real order is the
// inspector's, checked by the root journeys).
func (f *fake) LayerOrder(worn []string) []int {
	idx := make([]int, len(worn))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int {
		switch {
		case worn[a] < worn[b]:
			return -1
		case worn[a] > worn[b]:
			return 1
		}
		return 0
	})
	return idx
}

func (f *fake) LayerTag(name string) string { return "Layer " + name + ": " }
func (f *fake) Insulation(kind, _, _ string) string {
	if kind == "parka" {
		return "Insulation: cold 22C, heat 3C"
	}
	return ""
}
func (f *fake) Note(kind string) string {
	if kind == "parka" {
		return "leather parka  (much warmer, hot in summer)"
	}
	return kind
}

// frameWith runs one kit frame with decl declaring the screen.
func (f *fake) frameWith(decl func()) {
	f.frame = inputframe.NewFrame(inputframe.Raw{Mx: -1, My: -1})
	f.kit.Begin()
	f.kit.RunModals(nil)
	decl()
}

// click declares a frame, clicks widget id and declares the next frame,
// which hands the click to the button.
func (f *fake) click(id string, decl func()) bool {
	f.frameWith(decl)
	ok := f.kit.Click(id)
	f.frameWith(decl)
	return ok
}
