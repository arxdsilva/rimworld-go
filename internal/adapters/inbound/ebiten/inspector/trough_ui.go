package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Feeding trough (plan 6.2, PLAY-018): the pane shows a fill bar, what the
// trough accepts and the pen's tame animals; one command button holds the
// Plants / Meat / Any filter (SetTroughFilter) as options.

const TroughKind = world.TroughKind

// troughFilters are the filter buttons; Mode is the SetTroughFilter value.
var troughFilters = []struct {
	key, label string
	mode       uint8
}{
	{"plants", "Plants", 0},
	{"meat", "Meat", 1},
	{"any", "Any", 2},
}

const MaxTroughAnimals = 6

func IsTrough(b game.BuildingView) bool { return b.Kind == TroughKind }

// troughCommands is the trough's filter button. Its options are the three
// filters, the current one disabled, so a left-click switches to the first
// other filter and a right-click picks.
func (a Building) troughCommands(b game.BuildingView) []Button {
	if !IsTrough(b) {
		return nil
	}
	opts := make([]Option, 0, len(troughFilters))
	for _, f := range troughFilters {
		o := Option{Label: f.label, Cmd: &command.Command{Kind: command.SetTroughFilter, Target: uint64(b.ID), Mode: f.mode}}
		if b.TroughFilter == f.key {
			o.Disabled, o.Reason = true, "Already set"
		}
		opts = append(opts, o)
	}
	desc := "Which feed haulers put in."
	if b.TroughFilter != "" {
		desc += " Now: " + b.TroughFilter + "."
	}
	return []Button{{ID: "cmd.bldg.trough", Label: "Feed", Options: func() []Option { return opts },
		Desc: desc + " Click to switch; right-click to pick."}}
}

// penAnimals are the tame animals standing in the pen that holds tile
// (x, y): the trough's herd. The trough itself blocks movement, so it is
// not a pen cell: the pen is the one holding a cell next to it.
func PenAnimals(w game.WorldView, x, y int) []game.AnimalView {
	var bits []uint64
	for _, p := range w.Pens {
		if !p.Enclosed {
			continue
		}
		for dy := -1; dy <= 1 && bits == nil; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if bitAt(p.Bits, w.TerrainW, x+dx, y+dy) {
					bits = p.Bits
					break
				}
			}
		}
		if bits != nil {
			break
		}
	}
	if bits == nil {
		return nil
	}
	var out []game.AnimalView
	for _, an := range w.Animals {
		if an.Tame && bitAt(bits, w.TerrainW, an.X, an.Y) {
			out = append(out, an)
		}
	}
	return out
}

// troughLines are the trough's pane lines: what it takes and its herd.
func (a Building) troughLines(b game.BuildingView) []string {
	if !IsTrough(b) {
		return nil
	}
	var out []string
	if b.TroughFilter != "" {
		out = append(out, "Accepts: "+b.TroughFilter)
	}
	list := PenAnimals(a.shell.World(), b.X, b.Y)
	out = append(out, fmt.Sprintf("Pen animals (%d)", len(list)))
	for i, an := range list {
		if i >= MaxTroughAnimals {
			out = append(out, fmt.Sprintf("... and %d more", len(list)-i))
			break
		}
		line := fmt.Sprintf("%s (%s)", an.Name, AnimalLabel(an))
		if AnimalHasNeeds(an) {
			line += fmt.Sprintf("  food %d%%", an.Food)
		}
		out = append(out, line)
	}
	return out
}
