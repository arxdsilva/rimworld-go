package inspector

import (
	"fmt"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/command"
)

// PackRows are the Gear tab's pack lines (CSE-036): its weight against the
// pawn's limit, then what is in it. No rows for an empty pack.
func PackRows(p game.PawnView) []string {
	if len(p.Pack) == 0 {
		return nil
	}
	rows := make([]string, 0, len(p.Pack)+1)
	rows = append(rows, fmt.Sprintf("Pack  %.1f / %.1f kg", float64(p.PackGrams)/1000, float64(p.PackLimitGrams)/1000))
	for _, it := range p.Pack {
		rows = append(rows, fmt.Sprintf("  %s x%d", it.Name, it.Qty))
	}
	return rows
}

// UnloadID is the Gear tab's Unload / Cancel button of one row (CSE-038):
// what is "pack", "worn", "weapon" or "spare", and n the entry or garment.
func UnloadID(what string, n int) string { return fmt.Sprintf("gear.unload.%s.%d", what, n) }

const unloadBtnW = 56

// gearUnloadButtons declares a button on each row of the Gear tab that can be
// put away: the wielded weapon, a worn garment, the spare weapon and each pack
// entry. Pack rows say Cancel while marked. Only for a living colonist, and only
// while the habit is on. lines are the tab's text lines below the hint, drawn from
// row 0 at g.Hint plus a line.
func (a Tabs) gearUnloadButtons(p game.PawnView, r gfx.Rect, g GearLayout, lines []string) {
	if p.Status != "alive" || p.Hostile || p.Prisoner || !a.shell.World().Gameplay.On(gameplay.ManualUnload) {
		return
	}
	step := gfx.LineStep(gfx.Small)
	put := func(what string, n, mode, index int, y float32, marked bool) {
		h := step
		if what == "weapon" { // its row is a heading line, and the panel's top is right above it
			h = gfx.LineStep(gfx.Label)
		}
		rect := gfx.Rect{X: r.Right() - unloadBtnW, Y: y, W: unloadBtnW, H: h}
		if rect.Bottom() > r.Bottom() {
			return
		}
		label, tip := "Unload", "Unload\nPut this away: in a stockpile if the colony has room, else on the ground beside "+p.Name+"."
		if marked {
			label, tip = "Cancel", "Cancel\nLeave this in the pack."
		}
		if a.kit().Button(UnloadID(what, n), rect, label, uikit.BtnOpts{Tip: tip}) {
			a.shell.Enqueue(command.Command{Kind: command.MarkUnload, Pawn: uint64(p.ID), Mode: uint8(mode), Index: index, Enabled: !marked})
		}
	}
	if p.Weapon != "" && p.Weapon != "fists" {
		put("weapon", 0, 2, 0, r.Y, false)
	}
	for k, i := range GearLayerOrder(p.Apparel) {
		put("worn", i, 1, i, r.Y+g.Clothes+float32(k+1)*step, false)
	}
	rowY := func(line int) float32 { return r.Y + g.Hint + step + 6 + float32(line)*step }
	for i, l := range lines {
		if strings.HasPrefix(l, "Spare weapon: ") {
			put("spare", 0, 3, 0, rowY(i), false)
		}
	}
	first := len(lines) - len(PackRows(p)) + 1 // the pack's header is the line before its entries
	for i, it := range p.Pack {
		put("pack", int(it.ID), 0, int(it.ID), rowY(first+i), it.Unload)
	}
}
