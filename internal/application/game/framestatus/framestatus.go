// Package framestatus is the frame inspector's status line (UI-015, plan
// Part B): the line formats and the pure choice between them, moved out of
// the game package, which fills game.FrameView.Status from it. It is pure:
// no state, no host.
package framestatus

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
)

// Frame status lines (game.FrameView.Status), first match wins.
const (
	Moving     = "Moving %s here: needs no materials"
	Changing   = "Changing to %s" // an in-place material swap (PLAY-011)
	NoStock    = "No %s in stock"
	OnTheWay   = "Materials on the way"
	CompsWay   = "Components on the way"
	NeedComps  = "Needs %d components"
	NeedExtra  = "Needs %d %s" // AX-4: a secondary material (Def.Extra)
	NoHauler   = "Waiting for a hauler"
	BuiltBy    = "Being built by %s"
	NoBuilder  = "Waiting for a builder"
	craftHintF = ": %s one at a %s"
	HaulerLine = "%s (%d %s)"
)

// Of is frame fr's one-line status: incoming is its material on the way,
// free the material's unreserved stock, compIncoming the components on
// the way and builder the colonist building it ("" when none is).
func Of(fr construction.Frame, incoming, free, compIncoming int, builder string) string {
	switch {
	case fr.Reinstall != 0:
		return fmt.Sprintf(Moving, construction.DefOf(fr.Kind).Name)
	case !fr.Ready() && fr.Wood+incoming < fr.Need && free <= 0:
		return fmt.Sprintf(NoStock, fr.Stuff.String()) + CraftHint(fr.Stuff)
	case fr.Wood >= fr.Need && fr.Comp < fr.CompNeed && compIncoming > 0:
		return CompsWay
	case fr.Wood >= fr.Need && fr.Comp < fr.CompNeed:
		return fmt.Sprintf(NeedComps, fr.CompNeed-fr.Comp)
	case fr.Wood >= fr.Need && fr.Comp >= fr.CompNeed && !fr.ExtrasIn():
		return fmt.Sprintf(NeedExtra, haul.SupplyNeed(fr, haul.SupplyKind(fr)), haul.SupplyKind(fr))
	case !fr.Ready() && incoming > 0:
		return OnTheWay
	case !fr.Ready():
		return NoHauler
	case builder != "":
		return fmt.Sprintf(BuiltBy, builder)
	}
	return NoBuilder
}

// CraftHint tells where a crafted material is made (": sew one at a
// tailoring bench"), from the first offered recipe whose output it is;
// "" for gathered materials.
func CraftHint(k item.Kind) string {
	for _, d := range recipe.All() {
		if d.Output != k || d.Legacy || d.Name == "" || d.OutQty <= 0 {
			continue
		}
		verb, _, _ := strings.Cut(strings.ToLower(d.Label), " ")
		return fmt.Sprintf(craftHintF, verb, construction.DefOf(d.Bench).Name)
	}
	return ""
}
