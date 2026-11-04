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
	if k := FloorSprite(kind, stuff); k != "" { // floor_keys.go: every floor kind (AX-3)
		return k
	}
	switch m := sprite.StuffMaterial(stuff); {
	case kind == "wall" && m != "wood":
		return sprite.WallPrefix(stuff) + "_" + sprite.TwoDigits(mask)
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
	case "straw matting", "concrete", "paved tile", "steel tile", "silver tile", "gold tile", "sterile tile", "stone tile",
		"flagstone", "carpet floor": // AX-3, floor_keys.go
		return FloorSprite(kind, "")
	case overlay.BridgeKind:
		return overlay.BridgeKey(mask)
	case "double bed":
		return "furniture/bed_double_" + FacingOr(facing)
	case "bed":
		return "furniture/bed_basic_" + FacingOr(facing)
	case "sleeping spot":
		return "overlay/sleeping_spot_" + FacingOr(facing)
	case "bedroll": // BED-1
		return "furniture/bedroll_" + FacingOr(facing)
	case "double bedroll":
		return "furniture/bedroll_double_" + FacingOr(facing)
	case "animal sleeping spot":
		return "overlay/animal_sleeping_spot"
	case "animal bed":
		return "furniture/animal_bed"
	case "end table":
		return "furniture/end_table"
	case "dresser":
		return "furniture/dresser_" + FacingOr(facing)
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
	case "ring toss", "ur board", "chess table", "poker table": // A6 (AR3's art)
		return "furniture/" + strings.ReplaceAll(kind, " ", "_")
	case "billiards table": // north draws south, west draws east
		return "furniture/billiards_table_" + axisFacing(facing)
	case "tube television", "flatscreen television", "big screen television", "telescope":
		return "furniture/" + strings.ReplaceAll(kind, " ", "_") + "_" + FacingOr(facing)
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
	case "table": // A2 (AR1's art): the 1x2 table; north draws south, west draws east
		return "furniture/table_1x2_" + axisFacing(facing)
	case "table 2x2":
		return "furniture/table_2x2"
	case "table 2x4":
		return "furniture/table_2x4_" + axisFacing(facing)
	case "table 3x3":
		return "furniture/table_3x3"
	case "ornate door", "animal flap":
		return "structure/" + strings.ReplaceAll(kind, " ", "_") + "_" + map[bool]string{true: "ew", false: "ns"}[ew] + "_" +
			map[bool]string{true: "open", false: "closed"}[open]
	case "double sleeping spot":
		return "overlay/double_sleeping_spot_" + FacingOr(facing)
	case "couch", "small shelf", "wall torch", "wall lamp":
		return "furniture/" + strings.ReplaceAll(kind, " ", "_") + "_" + FacingOr(facing)
	case "column", "animal sleeping box", "plant pot", "flood light", "urn", "large stele", "grand stele", "egg box":
		return "furniture/" + strings.ReplaceAll(kind, " ", "_")
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
	case "ship structural beam", "ship stasis casket", "ship engine", "ship computer core", "ship reactor", "ship sensor cluster":
		return ShipSprite(kind, facing) // ship_keys.go (AX-8)
	case StasisCasketKind, TransportPodKind, PodLauncherKind, "long-range mineral scanner", "caravan hitching spot":
		return TravelSprite(kind, facing) // travel_keys.go (AX-9)
	case env.SteamVentKind:
		return "structure/steam_vent"
	case "pen marker":
		return "structure/pen_marker"
	case "crafting spot": // wave D
		return "overlay/crafting_spot"
	case "butchering spot":
		return "overlay/butcher_spot"
	case "wood-fired forge":
		return "furniture/fueled_smithy_" + FacingOr(facing)
	case "electric forge":
		return "furniture/electric_smithy_" + FacingOr(facing)
	case "machining table":
		return "furniture/machining_table_" + FacingOr(facing)
	case "fabrication bench":
		return "furniture/fabrication_bench_" + FacingOr(facing)
	case "sculpting bench":
		return "furniture/art_bench_" + FacingOr(facing)
	case "electric smelter":
		return "furniture/electric_smelter_" + FacingOr(facing)
	case "fuel refinery":
		return "furniture/biofuel_refinery_" + FacingOr(facing)
	case "fermenting barrel":
		return "furniture/fermenting_barrel"
	case "small sculpture":
		return "furniture/sculpture_small"
	case "large sculpture":
		return "furniture/sculpture_large"
	case "grand sculpture":
		return "furniture/sculpture_grand"
	case "stool":
		return "furniture/stool"
	case "armchair":
		return "furniture/armchair_" + FacingOr(facing)
	case "shelf": // PRD-4
		return "furniture/shelf_" + FacingOr(facing)
	case BookcaseKind, SmallBookcaseKind: // A7 (AR3's art): empty; BookcaseKey picks the filled looks
		return BookcaseKey(kind, facing, 0)
	case "display case", "round pedestal", "small display shelf", "display shelf", "weapon rack",
		"narrow wardrobe", "wardrobe", "narrow display cabinet", "display cabinet", "open shelf",
		"two tier display shelf", "narrow tall shelf", "tall shelf", "safe", "drum",
		"horizontal weapon rack", "vertical weapon rack", "textile rack", "meat hook",
		"mini pallet", "half pallet", "pallet", "small crate", "crate", "large crate", "tall crate",
		"corpse dump spot": // NST: neat_keys.go
		return NeatKey(kind, facing)
	case "hopper": // STO-2
		return "furniture/hopper"
	case "nutrient paste dispenser":
		return "furniture/nutrient_paste_dispenser"
	case "sarcophagus":
		return "furniture/sarcophagus_" + FacingOr(facing)
	case "hydroponics basin": // FOOD-2: faced; its glow layer comes from DrawPowerParts
		return "furniture/hydroponics_basin_" + FacingOr(facing)
	case FishTankKind: // FISH: faced like the basin; glow and fish overlays come from DrawPowerParts and drawTankFish
		return "furniture/fish_tank_" + FacingOr(facing)
	case "sun lamp":
		return "furniture/sun_lamp"
	case "hive mound": // CBT-4
		return "structure/hive_mound"
	case "ship part": // DEF-4: not buildable, drawn where it crashed
		return "defense/ship_part"
	case "barricade":
		return "defense/barricade"
	case "blast charge":
		return "defense/ied_high_explosive"
	case "fire charge":
		return "defense/ied_incendiary"
	case "pulse charge":
		return "defense/ied_emp"
	case "smoke charge":
		return "defense/ied_smoke"
	case "autocannon":
		return "defense/autocannon_base"
	case "slug turret":
		return "defense/slug_turret_base"
	case "mortar":
		return "defense/mortar_base"
	case "foam popper": // AX-5 (AR2's art)
		return "defense/foam_popper"
	case "foam charge":
		return "defense/ied_foam"
	case "warhead charge":
		return "defense/ied_warhead"
	case overlay.FoamTurretKind:
		return "defense/foam_turret_base"
	case overlay.RocketBatteryKind:
		return "defense/rocket_battery_base"
	case "powered door":
		return "structure/autodoor_" + map[bool]string{true: "ew", false: "ns"}[ew] + "_" + map[bool]string{true: "open", false: "closed"}[open]
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
	case "vent": // TEMP-1: open lets air through (BuildingView.Open)
		if open {
			return "furniture/vent_open_" + FacingOr(facing)
		}
		return "furniture/vent_closed_" + FacingOr(facing)
	case "passive cooler": // TEMP-1: open is fuelled here
		if open {
			return "furniture/passive_cooler"
		}
		return "furniture/passive_cooler_empty"
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
	case "multi-analyzer": // RSCH-3: 2x2 facility, one facing; its glow is the on state
		return "furniture/multi_analyzer"
	case overlay.WaterproofConduitKind: // AX-4 (AR2's art)
		return "power/waterproof_conduit_" + sprite.TwoDigits(mask&15)
	case "chemfuel generator":
		return "power/chemfuel_generator"
	case overlay.WatermillKind: // the wheel is its own layer (power_draw.go drawMillWheel)
		return "power/watermill_" + FacingOr(facing)
	case "deep drill":
		return "power/deep_drill_" + FacingOr(facing)
	case "ground-penetrating scanner":
		return "power/ground_scanner_" + FacingOr(facing)
	case "moisture pump":
		return "power/moisture_pump"
	case "electric crematorium":
		return "furniture/electric_crematorium_" + FacingOr(facing)
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

// axisFacing is south or east: the facing of a kind whose north and
// west draw as its south and east (a table).
func axisFacing(f string) string {
	if f == "east" || f == "west" {
		return "east"
	}
	return "south"
}

// FacingOr is f, south when unset.
func FacingOr(f string) string {
	if f == "" {
		return "south"
	}
	return f
}
