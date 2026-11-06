package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
)

// AX-2 structure, furniture and decor on the building inspector, read
// from BuildingView.AX (game/axview): a table's or couch's seats, a plant
// pot's plant and its sow buttons, a hung light, and what a column, an
// animal flap and an egg box do.

// AX-2 building kinds (construction names).
const (
	plantPotKind   = "plant pot"
	ornateDoorKind = "ornate door"
	smallShelfKind = "small shelf"
	eggBoxKind     = "egg box"
)

// PotSowPrefix starts a plant pot's sow buttons (cmd.pot.<plant>).
const PotSowPrefix = "cmd.pot."

// ax2Lines are the seats, the pot's plant, the hung light's wall and the
// rule a column, flap or egg box follows.
func ax2Lines(b game.BuildingView) []string {
	var out []string
	if b.AX.Seats > 1 {
		out = append(out, fmt.Sprintf("Seats %d", b.AX.Seats))
	}
	switch {
	case b.Kind == plantPotKind && b.AX.Plant != "" && b.AX.PlantPct == 0:
		out = append(out, capital(b.AX.Plant)+" chosen: waiting for a grower to sow it")
	case b.Kind == plantPotKind && b.AX.Plant != "":
		out = append(out, fmt.Sprintf("%s, %d%% grown", capital(b.AX.Plant), b.AX.PlantPct), fmt.Sprintf("Beauty %+.1f", float64(b.Beauty)/1000))
	case b.Kind == plantPotKind:
		out = append(out, "Empty: sow a plant below")
	case b.AX.WallMounted:
		out = append(out, "Hung on the wall behind it")
	case b.Kind == "column":
		out = append(out, "Holds a roof up to 6 tiles away")
	case b.Kind == "animal flap":
		out = append(out, "Tame animals pass; people and wild animals don't")
	case b.Kind == eggBoxKind:
		out = append(out, "Tame hens within 20 tiles lay here")
	}
	return out
}

// potCommands are a plant pot's sow buttons, one per plant: the plant it
// holds is on, and clicking it empties the pot.
func potCommands(tr lang.Tr, b game.BuildingView) []Button {
	if b.Kind != plantPotKind {
		return nil
	}
	var out []Button
	for _, p := range construction.PotPlants() {
		name := p.DefOf().Name
		on, next := b.AX.Plant == name, p
		if on {
			next = construction.PotNone
		}
		out = append(out, Button{ID: PotSowPrefix + name, Label: tr.T("Sow ") + name, Icon: "plant_pot", Toggle: true, On: on,
			Desc: lang.Tf(tr, "Have a grower sow a %s here (beauty %d when grown). It grows in light; frost kills it and it is sown again. On: click to empty the pot.", name, p.DefOf().Beauty),
			Cmds: []command.Command{{Kind: command.SetPotPlant, Target: uint64(b.ID), Build: uint8(next)}}})
	}
	return out
}

// capital is s with its first letter upper case.
func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
