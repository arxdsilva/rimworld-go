package construction

// axMaxHP is the base hit points of an AX-0 contract kind (ok=false: the
// default). wiki: each item's infobox (the AX-0 wiki notes); a steel-only
// kind's base is the wiki value divided by steel's 140 % (as DEF-3), so
// steel gives the wiki figure (the ship reactor's 500 too: wiki: Ship
// reactor). Floors keep the default.
func axMaxHP(k Kind) (int, bool) {
	switch k {
	case OrnateDoor, BilliardsTable, PokerTable:
		return 250, true
	case Column, Couch:
		return 160, true
	case AnimalFlap:
		return 20, true
	case AnimalSleepingBox, PlantPot, Table: // wiki: Table (1x2) 75
		return 75, true
	case Table2x2, RingToss, ChessTable, Bookcase,
		DisplayShelf, WeaponRack, Wardrobe, DisplayCabinet, OpenShelf, TwoTierDisplayShelf, TallShelf, HorizontalWeaponRack, VerticalWeaponRack, MeatHook, HalfPallet, Pallet, Crate, LargeCrate, TallCrate: // NST 2x1 and 2x2, ours
		return 100, true
	case Table2x4:
		return 150, true
	case Table3x3:
		return 175, true
	case SmallShelf, WallTorch, EggBox, SmallBookcase,
		DisplayCase, RoundPedestal, SmallDisplayShelf, NarrowWardrobe, NarrowDisplayCabinet, NarrowTallShelf,
		Safe, Drum, TextileRack, MiniPallet, SmallCrate, CorpseDumpSpot: // NST 1x1, ours
		return 50, true
	case Urn, IEDFoam, IEDWarhead:
		return 40, true
	case LargeStele:
		return 200, true
	case GrandStele:
		return 400, true
	case UrBoard:
		return 90, true
	case Watermill:
		return 400, true
	case ElectricCrematorium:
		return 300, true
	case WallLamp: // 35
		return 25, true
	case FloodLight: // 120
		return 86, true
	case WaterproofConduit: // 80
		return 57, true
	case ChemfuelGenerator: // 300
		return 214, true
	case GroundScanner, RocketBattery, PodLauncher, ShipCasket: // 200
		return 143, true
	case MoisturePump, FoamPopper: // 50
		return 36, true
	case FoamTurret, TubeTV, ShipSensor: // 100
		return 71, true
	case FlatscreenTV: // 80
		return 57, true
	case BigScreenTV: // 85
		return 61, true
	case Telescope: // 75
		return 54, true
	case ShipBeam, ShipEngine: // 400
		return 286, true
	case ShipCore: // 150
		return 107, true
	case ShipReactor: // 500
		return 357, true
	case StasisCasket, TransportPod, MineralScanner: // 250
		return 179, true
	}
	return 0, false
}
