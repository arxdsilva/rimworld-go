package power

import "rimworld-go/internal/domain/building/construction"

// WTicksPerWh is how many W·ticks make one Wh (one game hour of 1 W:
// clock.TicksPerHour).
const WTicksPerWh = 2500

// ConnectRange is how far (Chebyshev, in tiles, from its footprint) a
// device reaches for a transmitter to join.
const ConnectRange = 3

// Balance rules (PLAY021_PLAN.md Balance; our numbers).
const (
	// ShedTicks is the least time a consumer shed in a brownout stays off
	// (one game hour).
	ShedTicks = 2500
	// HeadroomPct is the margin a net must have over its load plus a shed
	// consumer's before that consumer comes back.
	HeadroomPct = 10
	// ChargePct is the share of a surplus a battery keeps.
	ChargePct = 75
	// SelfDischargeW is what each battery leaks.
	SelfDischargeW = 8
)

// Role is what a building does on a power grid.
type Role uint8

const (
	None        Role = iota // not electrical
	Producer                // makes power (generators, solar, wind, geothermal)
	Consumer                // draws power (lamps, heaters, coolers, electric benches)
	Storage                 // stores power (batteries)
	Transmitter             // carries power between neighbours (conduits)
	Switch                  // a transmitter while closed; open, it cuts the grid
)

// String is the role's view name ("" for None).
func (r Role) String() string {
	switch r {
	case Producer:
		return "producer"
	case Consumer:
		return "consumer"
	case Storage:
		return "storage"
	case Transmitter:
		return "transmitter"
	case Switch:
		return "switch"
	}
	return ""
}

// Class is a consumer's place in the shed order of a brownout: the lowest
// class is switched off first (lamps, then benches, coolers, heaters, and
// defences last).
type Class uint8

const (
	ClassNone Class = iota // not a consumer
	ClassLamp
	ClassBench
	ClassCooler
	ClassHeater
	// ClassDefense (PLAY-024): sentry turrets, shed last in a brownout.
	ClassDefense
)

// Spec is a building kind's electrical definition (our numbers). Draw is
// the W a consumer uses while working, Idle while idle; Peak the most W a
// producer makes; CapWh what a battery holds. Toggle: the player can
// switch it off; Hidden: a buried conduit (drawn only in the overlay,
// never chosen by a short circuit).
type Spec struct {
	Role                    Role
	Class                   Class
	Draw, Idle, Peak, CapWh int
	Toggle, Hidden          bool
}

// SpecOf is kind k's electrical definition (the zero Spec, Role None, for
// a kind that is not electrical, the torch lamp included). A lamp has no
// idle state: it draws its 50 W whenever it is on. The electric benches
// idle at a tenth of their working draw.
func SpecOf(k construction.Kind) Spec {
	switch k {
	case construction.PowerConduit:
		return Spec{Role: Transmitter}
	case construction.HiddenConduit:
		return Spec{Role: Transmitter, Hidden: true}
	case construction.PowerSwitch:
		return Spec{Role: Switch, Toggle: true}
	case construction.WoodGenerator:
		return producer(800)
	case construction.SolarPanel:
		return producer(900)
	case construction.WindTurbine:
		return producer(1100)
	case construction.GeothermalGenerator:
		return producer(1800)
	case construction.Battery:
		return Spec{Role: Storage, CapWh: 1200}
	case construction.Lamp:
		return consumer(ClassLamp, 50, 50)
	case construction.Heater:
		return consumer(ClassHeater, 180, 20)
	case construction.Cooler:
		return consumer(ClassCooler, 220, 20)
	case construction.ElectricStove:
		return bench(250)
	case construction.ElectricTailoringBench:
		return bench(150)
	case construction.ElectricResearchBench:
		return bench(200)
	case construction.VitalsMonitor: // wiki: Vitals monitor (80 W)
		return consumer(ClassBench, 80, 80)
	case construction.SentryTurret: // PLAY-024 (our numbers): 120 W firing, 40 W idle
		return consumer(ClassDefense, 120, 40)
	case construction.RadioConsole: // PLAY-024: 150 W while worked, 15 W idle
		return bench(150)
	// Wave D (ENG-003, wiki watts; a bench idles at a tenth, our rule).
	case construction.ElectricSmithy: // wiki: Electric smithy
		return bench(210)
	case construction.MachiningTable: // wiki: Machining table
		return bench(350)
	case construction.FabricationBench: // wiki: Fabrication bench
		return bench(250)
	case construction.ElectricSmelter: // wiki: Electric smelter
		return bench(700)
	case construction.BiofuelRefinery: // wiki: Biofuel refinery
		return bench(170)
	case construction.Autocannon, construction.UraniumSlugTurret: // wiki: Autocannon turret, Uranium slug turret (no idle figure: the same)
		return consumer(ClassDefense, 150, 150)
	case construction.Autodoor: // wiki: Autodoor (shed with the defences)
		return consumer(ClassDefense, 50, 50)
	case construction.NutrientPasteDispenser: // STO-2, wiki: Nutrient paste dispenser (200 W)
		return consumer(ClassBench, 200, 200)
	case construction.HydroponicsBasin: // FOOD-2, wiki: Hydroponics basin (70 W day and night)
		return consumer(ClassBench, 70, 70)
	case construction.SunLamp: // FOOD-2, wiki: Sun lamp (2,900 W, only 06:00-19:12: the game's powerWant)
		return consumer(ClassLamp, 2900, 0)
	}
	return Spec{}
}

func producer(peak int) Spec { return Spec{Role: Producer, Peak: peak, Toggle: true} }

func consumer(c Class, draw, idle int) Spec {
	return Spec{Role: Consumer, Class: c, Draw: draw, Idle: idle, Toggle: true}
}

func bench(draw int) Spec { return consumer(ClassBench, draw, draw/10) }
