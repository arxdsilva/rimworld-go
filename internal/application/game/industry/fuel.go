package industry

import (
	"rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
)

// The shared fuel table (PLAY-021, moved here from game/fuel.go by AX-4):
// everything that burns logs or chemfuel. The game holds each burner's
// fuel as ticks of burning, burns it in its climate phase and has haulers
// feed it; this is how much each kind holds and how fast it burns.

const (
	// TicksPerWood is how long one log burns (the game's TicksPerWood).
	TicksPerWood = 6000
	// CampfireLogs is how much wood a campfire holds (the game's
	// CampfireCapacity).
	CampfireLogs = 20
	// TorchLogs and GeneratorLogs are how many logs a full tank takes
	// (wiki: Torch lamp 20, Wood-fired generator 75; unscaled, like the
	// campfire's 20).
	TorchLogs     = 20
	GeneratorLogs = 75
	// TorchBurn and GeneratorBurn are the ticks of fuel used per 1000 ticks
	// of burning (campfire: 1000, one for one, its wiki 10 logs a day):
	// wiki: Torch lamp 2 logs a day, Wood-fired generator 22 a day.
	TorchBurn     = 200
	GeneratorBurn = 2200
	// ChemfuelTicks and ChemfuelTank, wiki: Chemfuel powered generator: 30
	// chemfuel a tank, 4.5 a day while running (one chemfuel burns 13333
	// ticks, independent of the load).
	ChemfuelTicks = 60000 * 2 / 9
	ChemfuelTank  = 30
)

// Fuel is how one kind of building holds and burns fuel.
type Fuel struct {
	// CapTicks is a full tank, in ticks of fuel.
	CapTicks int
	// Below: haulers bring more once the tank falls under this.
	Below int
	// BurnPermil is the fuel used per 1000 ticks of burning: 1000 burns
	// one tick of fuel per tick, 1600 one and a half times as fast.
	BurnPermil int
	// Banks: the fire banks down in warm air (campfires only).
	Banks bool
}

// FuelSpec is kind k's fuel table row; ok is false for a kind that burns
// nothing.
func FuelSpec(k construction.Kind) (Fuel, bool) {
	switch k {
	case construction.Campfire:
		return Fuel{CapTicks: CampfireLogs * TicksPerWood, Below: CampfireLogs * TicksPerWood / 2, BurnPermil: 1000,
			Banks: true}, true
	case construction.TorchLamp:
		return Fuel{CapTicks: TorchLogs * TicksPerWood, Below: TorchLogs * TicksPerWood / 2, BurnPermil: TorchBurn}, true
	case construction.WallTorch: // A2, wiki: Wall torch lamp (holds 15 wood, burns 2 a day)
		return Fuel{CapTicks: 15 * TicksPerWood, Below: 15 * TicksPerWood / 2, BurnPermil: TorchBurn}, true
	case construction.WoodGenerator:
		return Fuel{CapTicks: GeneratorLogs * TicksPerWood, Below: GeneratorLogs * TicksPerWood / 2, BurnPermil: GeneratorBurn}, true
	case construction.ChemfuelGenerator:
		return Fuel{CapTicks: ChemfuelTank * ChemfuelTicks, Below: ChemfuelTank * ChemfuelTicks / 2, BurnPermil: 1000}, true
	case construction.PodLauncher: // AX-9, wiki: Pod launcher (150 chemfuel; burns only on a launch, game/travel)
		return Fuel{CapTicks: 150 * ChemfuelTicks, Below: 135 * ChemfuelTicks}, true
	case construction.FueledSmithy, construction.Stove: // PRD-3, FOOD-4, wiki: Fueled smithy, Fueled stove (50 logs, scaled / 10); burns only while worked (craft.WorkPerWood)
		return Fuel{CapTicks: 5 * TicksPerWood, Below: 5 * TicksPerWood / 2}, true
	case construction.PassiveCooler: // TEMP-1, wiki: Passive cooler (50 wood, 10 a day: one per TicksPerWood)
		return Fuel{CapTicks: 50 * TicksPerWood, Below: 25 * TicksPerWood, BurnPermil: 1000}, true
	case construction.FishTank: // FISH-*, original (domain/building/aqua): feed, a quarter asks, burns only while stocked and powered (game.burning)
		return Fuel{CapTicks: aqua.FeedFull, Below: aqua.FeedBelow, BurnPermil: 1000}, true
	}
	return Fuel{}, false
}

// oilWood: one ember oil burns as long as this many logs (SIM-026).
const oilWood = 3

// FuelTicks is how long one item of kind k burns (0: not a fuel): a log
// TicksPerWood, an ember oil oilWood logs' worth.
func FuelTicks(k item.Kind) int {
	switch k {
	case item.Wood:
		return TicksPerWood
	case item.EmberOil:
		return oilWood * TicksPerWood
	case item.Chemfuel:
		return ChemfuelTicks
	case item.Hay, item.Kibble: // FISH-*: fish feed, as raw plant food below
		return aqua.FeedItemTicks
	}
	if (recipe.Input{AnyVeg: true}).Accepts(k) {
		return aqua.FeedItemTicks
	}
	return 0
}
