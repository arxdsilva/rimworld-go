package world

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
)

// The building art keys: the world sprite of every building kind, with
// its material (walls, floors, doors, fences, carpets), its facing and
// its link mask, and the key the pipeline draws (BuildKey: the material's
// art, else the wood sprite).

// BuildKey is BuildSpriteStuff with a fallback to the wood sprite when a
// material's art (steel doors, stone fences) is missing.
func (b *base) BuildKey(kind, stuff, facing string, mask int, ew, open bool) string {
	key := BuildSpriteStuff(kind, stuff, facing, mask, ew, open)
	if b.art() != nil && sprite.StuffMaterial(stuff) != "wood" && !b.art().Has(key) {
		if alt := BuildSprite(kind, facing, mask, ew, open); alt != "" && b.art().Has(alt) {
			return alt
		}
	}
	return key
}

// BuildSpriteStuff is BuildSprite with a material (walls and floors).
func BuildSpriteStuff(kind, stuff, facing string, mask int, ew, open bool) string {
	switch kind {
	case sprite.FenceKind:
		return sprite.FenceSprite(stuff, mask)
	case sprite.GateKind:
		return GateSprite(facing, open)
	}
	if kind == "carpet" {
		return CarpetSprite(stuff)
	}
	switch m := sprite.StuffMaterial(stuff); {
	case kind == "wall" && m != "wood":
		return sprite.WallPrefix(stuff) + "_" + sprite.TwoDigits(mask)
	case kind == "floor" && m == "stone":
		return "structure/floor_stone"
	case kind == "door" && m == "steel":
		return DoorSprite("steel", ew, open)
	}
	return BuildSprite(kind, facing, mask, ew, open)
}

// CarpetSprite is furniture/carpet_<straw|cloth|leather|wood> for a carpet
// material ("cloth carpet" or just "cloth"); straw when unknown.
func CarpetSprite(stuff string) string {
	m := strings.TrimSuffix(stuff, " carpet")
	switch m {
	case "straw", "cloth", "leather", "wood", "wool":
		return "furniture/carpet_" + m
	}
	return "furniture/carpet_straw"
}

// BuildSprite is the world sprite for a building kind; neighbours only
// matter for walls and conduits (autotile) and doors (axis). open draws
// doors open and a power switch off.
func BuildSprite(kind, facing string, mask int, ew, open bool) string {
	switch kind {
	case sprite.FenceKind:
		return sprite.FenceSprite("wood", mask)
	case sprite.GateKind:
		return GateSprite(facing, open)
	case "wall":
		return "structure/wall_wood_" + sprite.TwoDigits(mask)
	case "door":
		return DoorSprite("wood", ew, open)
	case "floor":
		return "structure/floor_wood"
	case overlay.BridgeKind:
		return overlay.BridgeKey(mask)
	case "double bed":
		return "furniture/bed_double_" + FacingOr(facing)
	case "bed":
		return "furniture/bed_basic_" + FacingOr(facing)
	case "sleeping spot":
		return "overlay/sleeping_spot_" + FacingOr(facing)
	case "marriage spot":
		return "overlay/marriage_spot_" + FacingOr(facing)
	case "party spot":
		return "overlay/party_spot"
	case "brewing bench":
		return "furniture/brewing_bench_" + FacingOr(facing)
	case "drug table":
		return "furniture/drug_table_" + FacingOr(facing)
	case "workbench":
		return "furniture/workbench_basic_" + FacingOr(facing)
	case "stove":
		return "furniture/stove_basic_" + FacingOr(facing)
	case "butcher table":
		return "furniture/butcher_table_" + FacingOr(facing)
	case "campfire":
		return "furniture/campfire"
	case "grave":
		return "furniture/grave_dug_" + map[bool]string{true: "east", false: "south"}[facing == "east" || facing == "west"]
	case "funeral pyre":
		return "furniture/pyre"
	case "horseshoe pin":
		return "furniture/horseshoe_pin"
	case "sandbags":
		return "furniture/sandbags"
	case "research bench":
		return "furniture/research_bench_" + FacingOr(facing)
	case "spike trap":
		return "furniture/spike_trap"
	case "tailoring bench":
		return "furniture/tailoring_bench_" + FacingOr(facing)
	case "drying rack":
		return "furniture/drying_rack_" + FacingOr(facing)
	case "table":
		return "furniture/table_small"
	case "chair":
		return "furniture/chair_basic_" + FacingOr(facing)
	case "lamp":
		return "furniture/lamp_standing"
	case "carpet":
		return CarpetSprite("")
	case "feeding trough":
		return "furniture/feeding_trough_empty_" + FacingOr(facing)
	case "stonecutter table":
		return "furniture/stonecutter_table_" + FacingOr(facing)
	case "tool cabinet":
		return "furniture/tool_cabinet"
	case "hospital bed":
		return "furniture/hospital_bed_" + FacingOr(facing)
	case "vitals monitor": // MED-06
		return "furniture/vitals_monitor_" + FacingOr(facing)
	case "pod shell":
		return "structure/pod_shell"
	case env.SteamVentKind:
		return "structure/steam_vent"
	case "pen marker":
		return "structure/pen_marker"
	}
	return PowerSprite(kind, facing, mask, open)
}

// PowerSprite is BuildSprite for the PLAY-021 kinds: conduits by link
// mask (power_draw.go), a switch on or (open circuit) off, the rest by
// facing where they rotate.
func PowerSprite(kind, facing string, mask int, open bool) string {
	switch kind {
	case overlay.ConduitKind:
		return "power/power_conduit_" + sprite.TwoDigits(mask&15)
	case overlay.HiddenConduitKind:
		return "power/hidden_conduit_" + sprite.TwoDigits(mask&15)
	case overlay.PowerSwitchKind:
		if open {
			return "power/power_switch_off"
		}
		return "power/power_switch_on"
	case overlay.WoodGeneratorKind:
		return "power/wood_fired_generator"
	case overlay.SolarPanelKind:
		return "power/solar_panel"
	case overlay.WindTurbineKind:
		return "power/wind_turbine_" + FacingOr(facing)
	case overlay.GeothermalKind:
		return "power/geothermal_generator"
	case overlay.BatteryKind:
		return "power/battery_" + FacingOr(facing)
	case overlay.TorchLampKind:
		return "furniture/torch_lamp"
	case overlay.HeaterKind:
		return "power/heater"
	case overlay.CoolerKind:
		return "power/cooler_" + FacingOr(facing)
	case overlay.SentryTurretKind:
		return overlay.TurretBaseKey
	case overlay.TradeBeaconKind:
		return "power/trade_beacon"
	case overlay.RadioConsoleKind:
		return "power/radio_console_" + FacingOr(facing)
	case "electric stove":
		return "furniture/electric_stove_" + FacingOr(facing)
	case "electric tailoring bench":
		return "furniture/electric_tailoring_bench_" + FacingOr(facing)
	case "electric research bench":
		return "furniture/electric_research_bench_" + FacingOr(facing)
	}
	return ""
}

// DoorSprite is structure/door_<material>_<ns|ew>_<open|closed>.
func DoorSprite(material string, ew, open bool) string {
	axis, state := "ns", "closed"
	if ew {
		axis = "ew"
	}
	if open {
		state = "open"
	}
	return "structure/door_" + material + "_" + axis + "_" + state
}

// FacingOr is f, south when unset.
func FacingOr(f string) string {
	if f == "" {
		return "south"
	}
	return f
}
