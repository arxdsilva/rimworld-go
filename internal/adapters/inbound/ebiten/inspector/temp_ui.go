package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"sort"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/command"
)

// Temperature on the building inspector (TEMP-5): a vent's open / shut
// button (command.ToggleVent: a colonist flicks it, like a power toggle),
// the passive cooler's fuel line, and the heat a source gives or takes.
// Everything is read from BuildingView (HeatRate, Open, Fuel); nothing
// here knows the sim's rates.

const (
	ventKind          = "vent"
	passiveCoolerKind = "passive cooler"
	ventToggleCmdID   = "cmd.vent.toggle"
)

// isVent reports b is a wall vent.
func isVent(b game.BuildingView) bool { return b.Kind == ventKind }

// ventCommands is a vent's one button: open it to let air through the wall,
// shut it to seal the wall again.
func ventCommands(b game.BuildingView) []Button { return ventCommandsT(lang.English{}, b) }

// ventCommandsT is ventCommands in tr's language.
func ventCommandsT(tr lang.Tr, b game.BuildingView) []Button {
	if !isVent(b) {
		return nil
	}
	desc := tr.T("Open the vent so air flows between the rooms on either side. A colonist walks over and flicks it, so it takes a moment.")
	if b.Open {
		desc = tr.T("Shut the vent to seal the wall again. A colonist walks over and flicks it, so it takes a moment.")
	}
	return []Button{{ID: ventToggleCmdID, Label: tr.T("Open"), Icon: "power_toggle", Toggle: true, On: b.Open, Desc: desc,
		Cmds: []command.Command{{Kind: command.ToggleVent, Target: uint64(b.ID), Enabled: !b.Open}}}}
}

// HeatRateText is a heat rate in tenths of heat per second as "+2.1 heat/s".
func HeatRateText(rate int) string {
	sign := "+"
	if rate < 0 {
		sign, rate = "-", -rate
	}
	return fmt.Sprintf("%s%d.%d heat/s", sign, rate/10, rate%10)
}

// TempLines are b's temperature lines: whether a vent is open, what a
// passive cooler does with its wood, and a heat source's output (heaters,
// campfires, stoves, coolers and the rest).
func TempLines(b game.BuildingView) []string {
	var out []string
	switch {
	case isVent(b):
		if b.Open {
			out = append(out, "Open: the rooms either side share their air")
		} else {
			out = append(out, "Shut: no air passes")
		}
	case b.Kind == passiveCoolerKind:
		switch {
		case b.FuelPct == 0:
			out = append(out, "Out of wood: no cooling")
		case b.HeatRate < 0:
			out = append(out, "Burns wood to cool the room")
		default:
			out = append(out, "Holding at its floor temperature")
		}
	}
	if b.HeatRate != 0 && !isVent(b) {
		verb := "Heat output "
		if b.HeatRate < 0 {
			verb = "Cooling "
		}
		out = append(out, verb+HeatRateText(b.HeatRate))
	}
	return out
}

// ExposureStage names the stage of an exposure condition at severity
// permille ("shivering" .. "extreme"), from the body's own stage table.
func ExposureStage(kind body.CondKind, permille int) string {
	d := body.DefOfCond(kind)
	i := int(d.StageAt(int32(permille)))
	if i < len(d.Stages) {
		return d.Stages[i].Label
	}
	return ""
}

// ExposureLines are a colonist's, prisoner's or guest's temperature
// status lines: hypothermia and heatstroke with their stage, and the parts
// with an open frostbite.
func ExposureLines(p game.PawnView) []string {
	var out []string
	if p.Hypothermia > 0 {
		out = append(out, fmt.Sprintf("Hypothermia %d%% (%s)", p.Hypothermia/10, ExposureStage(body.CondHypothermia, p.Hypothermia)))
	}
	if p.Heatstroke > 0 {
		out = append(out, fmt.Sprintf("Heatstroke %d%% (%s)", p.Heatstroke/10, ExposureStage(body.CondHeatstroke, p.Heatstroke)))
	}
	if p.Frostbite > 0 {
		out = append(out, fmt.Sprintf("Frostbite on %d %s", p.Frostbite, plural(p.Frostbite, "part", "parts")))
	}
	if ln := ExposureNoteLine(p); ln != "" {
		out = append(out, ln)
	}
	return out
}

// ExposureNoteLine says why a downed colonist's hypothermia or heatstroke
// is not healing (AIL-004): "Not recovering: air 33.4 °C, needs 29.2 °C or
// cooler", from the view's air and comfortable range; "" while it heals
// (PawnView.ExposureNote empty). The sim's note is in whole °C; this line
// is built from the numbers.
func ExposureNoteLine(p game.PawnView) string {
	switch {
	case p.ExposureNote == "":
		return ""
	case p.Temp > p.ComfortMax:
		return "Not recovering: air " + FmtTarget(p.Temp) + ", needs " + FmtTarget(p.ComfortMax) + " or cooler"
	case p.Temp < p.ComfortMin:
		return "Not recovering: air " + FmtTarget(p.Temp) + ", needs " + FmtTarget(p.ComfortMin) + " or warmer"
	}
	return p.ExposureNote
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ExposureTip is the extra hover text of a Health-tab condition row that
// is an exposure condition ("" for the rest): what causes it and how it
// ends.
func ExposureTip(label string) string {
	switch {
	case strings.HasPrefix(label, "Hypothermia"):
		return "Caused by air colder than the colonist's comfortable range. Warm clothes, a fire or a heater, or a warmer room stop it growing; it fades once the air is comfortable again."
	case strings.HasPrefix(label, "Heatstroke"):
		return "Caused by air hotter than the colonist's comfortable range. A cooler, shade or lighter clothes stop it growing; it fades once the air is comfortable again."
	case strings.HasPrefix(label, "Hypothermic slowdown"):
		return "Cold-blooded creatures slow down in the cold instead of dying of it."
	}
	return ""
}

// AnimalTempLines are an animal's temperature lines: its comfortable range
// (wild and tame alike) and, once it is exposed, hypothermia and heatstroke
// with their stage. Nothing for a view with no range.
func AnimalTempLines(an game.AnimalView) []string {
	if an.ComfortMax <= an.ComfortMin {
		return nil
	}
	out := []string{"Comfortable " + FmtTarget(an.ComfortMin) + " to " + FmtTarget(an.ComfortMax)}
	if an.Hypothermia > 0 {
		out = append(out, fmt.Sprintf("Hypothermia %d%% (%s)", an.Hypothermia, ExposureStage(body.CondHypothermia, an.Hypothermia*10)))
	}
	if an.Heatstroke > 0 {
		out = append(out, fmt.Sprintf("Heatstroke %d%% (%s)", an.Heatstroke, ExposureStage(body.CondHeatstroke, an.Heatstroke*10)))
	}
	return out
}

// gearRank orders the worn layers, innermost first (wiki: Apparel layers).
var gearRank = map[item.Slot]int{item.SkinTorso: 0, item.SkinLegs: 1, item.Middle: 2, item.Legs: 3, item.Body: 4, item.Head: 5, item.Belt: 6}

// gearLayerNames name the layer of a slot for the Gear tab.
var gearLayerNames = map[item.Slot]string{item.SkinTorso: "Skin", item.SkinLegs: "Skin", item.Middle: "Middle", item.Legs: "Middle",
	item.Body: "Outer", item.Head: "Head", item.Belt: "Belt"}

func gearSlot(name string) (item.Slot, bool) {
	k, ok := item.ParseKind(name)
	if !ok || !k.IsApparel() {
		return item.NoSlot, false
	}
	return k.Slot(), true
}

// GearLayerOrder is the indices of worn in layer order (stable): the Gear
// tab lists what the colonist wears from the skin out.
func GearLayerOrder(worn []string) []int {
	idx := make([]int, len(worn))
	for i := range idx {
		idx[i] = i
	}
	rank := func(i int) int {
		if s, ok := gearSlot(worn[i]); ok {
			return gearRank[s]
		}
		return len(gearRank)
	}
	sort.SliceStable(idx, func(a, b int) bool { return rank(idx[a]) < rank(idx[b]) })
	return idx
}

// GearLayerTag is the "Skin: " style prefix of a worn garment's row ("" for
// a name that is not apparel).
func GearLayerTag(name string) string {
	if s, ok := gearSlot(name); ok {
		return gearLayerNames[s] + ": "
	}
	return ""
}

// InsulationLine is a garment's insulation as the item pane and the gear
// tab show it: "Insulation: cold 22.0 °C, heat 3.0 °C" (the degrees of
// comfort range it adds, from the garment, its material and its quality;
// wiki: Apparel). "" for anything that is not apparel or insulates nothing.
func InsulationLine(kind, stuff, qual string) string {
	cold, heat, ok := garmentInsulation(kind, stuff, qual)
	if !ok || cold == 0 && heat == 0 {
		return ""
	}
	return fmt.Sprintf("Insulation: cold %s, heat %s", FmtTarget(cold), FmtTarget(heat))
}

func garmentInsulation(kind, stuff, qual string) (cold, heat int, ok bool) {
	k, found := item.ParseKind(kind)
	if !found || !k.IsApparel() {
		return 0, 0, false
	}
	m, _ := item.ParseKind(stuff)
	var q uint8
	for _, l := range quality.All() {
		if l.String() == strings.ToLower(qual) {
			q = quality.Stored(l)
		}
	}
	cold, heat = item.Insulation(k, m, q)
	return cold, heat, true
}
