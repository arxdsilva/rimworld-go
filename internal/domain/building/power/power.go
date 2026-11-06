package power

import "rimworld-go/internal/domain/building/construction"

// WTicksPerWh is how many W·ticks make one Wh (one game hour of 1 W:
// clock.TicksPerHour).
const WTicksPerWh = 2500

// ConnectRange is how far (in tiles, from its footprint) a device reaches
// for a conductor to wire to (wiki: Power, 6 squares). The wiki doesn't
// publish how it measures; ours is Chebyshev.
const ConnectRange = 6

// Balance rules (.agents/handoffs/PLAY.md#play021_plan Balance; our numbers).
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
	Producer                // makes power (generators, solar, wind, geothermal); conducts
	Consumer                // draws power (lamps, heaters, coolers, electric benches); never conducts
	Storage                 // stores power (batteries); conducts
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

// Device: a role the balance works on (not a conduit or switch).
func (r Role) Device() bool { return r == Producer || r == Consumer || r == Storage }

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

// Thermostat: a heater or a cooler.
func (s Spec) Thermostat() bool { return s.Class == ClassHeater || s.Class == ClassCooler }

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
	case construction.Heater: // TEMP-1, wiki: Heater (175 W heating, 17 W idle)
		return consumer(ClassHeater, 175, 17)
	case construction.Cooler: // TEMP-1, wiki: Cooler (200 W, 20 W idle)
		return consumer(ClassCooler, 200, 20)
	case construction.Vent: // TEMP-1: flicked open or shut like a toggle; draws nothing
		return Spec{Toggle: true}
	case construction.ElectricStove:
		return bench(250)
	case construction.ElectricTailoringBench:
		return bench(150)
	case construction.ElectricResearchBench: // RSCH-1, wiki: Hi-tech research bench (250 W)
		return bench(250)
	case construction.MultiAnalyzer: // RSCH-1, wiki: Multi-analyzer (200 W, drawn all the time)
		return consumer(ClassBench, 200, 200)
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
	case construction.FishTank: // FISH-*: as the basin (70 W day and night)
		return consumer(ClassBench, 70, 70)
	case construction.SunLamp: // FOOD-2, wiki: Sun lamp (2,900 W, only 06:00-19:12: the game's powerWant)
		return consumer(ClassLamp, 2900, 0)
	}
	return axSpec(k) // ax.go: the AX-0 contract kinds
}

func producer(peak int) Spec { return Spec{Role: Producer, Peak: peak, Toggle: true} }

func consumer(c Class, draw, idle int) Spec {
	return Spec{Role: Consumer, Class: c, Draw: draw, Idle: idle, Toggle: true}
}

func bench(draw int) Spec { return consumer(ClassBench, draw, draw/10) }
