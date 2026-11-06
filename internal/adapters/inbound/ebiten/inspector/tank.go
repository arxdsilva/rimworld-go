package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Fish tank pane (FISH): a stock picker in the style of the trough's filter
// (left-click takes the first other stock, right-click or the corner arrow
// picks), the stock, growth, health, trouble and feed. It lives in its own
// type so Building takes no more methods: New wires tankProducer around the
// building producer.

// FishTankKind is the fish tank's construction name.
const FishTankKind = world.FishTankKind

// TankStockBtnID is the stock picker's command id.
const TankStockBtnID = "cmd.bldg.tank"

// fishDyingPrefix starts the pane line that names why the stock is dying;
// PaneLineWarns draws it in the warning colour.
const fishDyingPrefix = "Fish dying"

// tankProducer adds a fish tank's pane and button to the building producer.
type tankProducer struct{ b Building }

// Pane is the building's pane plus a tank's lines and feed bar.
func (t tankProducer) Pane(id entity.EntityID) Spec {
	p := t.b.BuildingPane(id)
	if bv, ok := t.b.BuildingView(id); ok && bv.Kind == FishTankKind && bv.Tank != nil {
		p.Bars = append(p.Bars, Bar{Label: t.b.shell.T("Feed"), Pct: bldgPct(bv.Tank.FeedPct), Color: uikit.Accent})
		p.Lines = append(p.Lines, TankLines(bv.Tank)...)
	}
	return p
}

// Commands is the building's commands with the stock picker first.
func (t tankProducer) Commands(id entity.EntityID) []Button {
	out := t.b.BuildingCommands(id)
	bv, ok := t.b.BuildingView(id)
	if !ok || bv.Kind != FishTankKind || bv.Tank == nil || bv.Hostile {
		return out
	}
	return append([]Button{TankStockButton(t.b.shell, bv)}, out...)
}

// TankLines are a tank's pane lines: stock, growth, the ready state, health
// and what is wrong.
func TankLines(t *game.TankView) []string {
	if t == nil {
		return nil
	}
	if !t.Stocked {
		if t.Stock == "" {
			return []string{"Stock: none chosen"}
		}
		return []string{"Stock: " + t.Stock + " (waiting to be stocked)"}
	}
	out := []string{"Stock: " + t.Stock}
	if t.Ripe {
		out = append(out, "Ready to net")
	} else {
		out = append(out, fmt.Sprintf("Growth  %d%%", t.GrowthPct))
	}
	out = append(out, fmt.Sprintf("Fish health  %d%%", t.HealthPct))
	if t.Trouble != "" {
		out = append(out, fishDyingPrefix+": "+t.Trouble)
	}
	return out
}

// TankStockButton is the stock picker of tank b: one option per species,
// the current one and any that Reason refuses disabled, and Empty.
func TankStockButton(tr lang.Tr, b game.BuildingView) Button {
	t := b.Tank
	opts := make([]Option, 0, len(t.Options)+1)
	for _, o := range t.Options {
		op := Option{Label: capitalize(o.Label), Cmd: &command.Command{Kind: command.SetTankStock, Target: uint64(b.ID), Count: o.ID}}
		switch {
		case o.ID == t.StockID && t.Stock != "":
			op.Disabled, op.Reason = true, tr.T("Already the stock")
		case o.Reason != "":
			op.Disabled, op.Reason = true, o.Reason
		}
		opts = append(opts, op)
	}
	if t.Stock != "" {
		opts = append(opts, Option{Label: tr.T("Empty"), Cmd: &command.Command{Kind: command.SetTankStock, Target: uint64(b.ID)}})
	}
	desc := tr.T("Which fish this tank raises.")
	if t.Stock != "" {
		desc += lang.Tf(tr, " Now: %s.", t.Stock)
	}
	return Button{ID: TankStockBtnID, Label: tr.T("Stock"), Icon: "fish_tank", Options: func() []Option { return opts },
		Desc: desc + tr.T(" Click to switch; right-click to pick.")}
}

// capitalize upper-cases the first letter of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
