package power

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
)

// TestSpecOf pins every kind's electrical definition (PLAY021_PLAN.md
// Model): conduits carry, the switch cuts, the generators' peaks, the
// battery's capacity, the consumers' draw and idle W and shed classes; the
// torch lamp and every other kind are not electrical.
func TestSpecOf(t *testing.T) {
	t.Parallel()
	want := map[construction.Kind]Spec{
		construction.PowerConduit:           {Role: Transmitter},
		construction.HiddenConduit:          {Role: Transmitter, Hidden: true},
		construction.PowerSwitch:            {Role: Switch, Toggle: true},
		construction.WoodGenerator:          {Role: Producer, Peak: 800, Toggle: true},
		construction.SolarPanel:             {Role: Producer, Peak: 900, Toggle: true},
		construction.WindTurbine:            {Role: Producer, Peak: 1100, Toggle: true},
		construction.GeothermalGenerator:    {Role: Producer, Peak: 1800, Toggle: true},
		construction.Battery:                {Role: Storage, CapWh: 1200},
		construction.Lamp:                   {Role: Consumer, Class: ClassLamp, Draw: 50, Idle: 50, Toggle: true},
		construction.Heater:                 {Role: Consumer, Class: ClassHeater, Draw: 180, Idle: 20, Toggle: true},
		construction.Cooler:                 {Role: Consumer, Class: ClassCooler, Draw: 220, Idle: 20, Toggle: true},
		construction.ElectricStove:          {Role: Consumer, Class: ClassBench, Draw: 250, Idle: 25, Toggle: true},
		construction.ElectricTailoringBench: {Role: Consumer, Class: ClassBench, Draw: 150, Idle: 15, Toggle: true},
		construction.ElectricResearchBench:  {Role: Consumer, Class: ClassBench, Draw: 200, Idle: 20, Toggle: true},
		construction.VitalsMonitor:          {Role: Consumer, Class: ClassBench, Draw: 80, Idle: 80, Toggle: true},
		construction.NutrientPasteDispenser: {Role: Consumer, Class: ClassBench, Draw: 200, Idle: 200, Toggle: true}, // STO-2
		construction.HydroponicsBasin:       {Role: Consumer, Class: ClassBench, Draw: 70, Idle: 70, Toggle: true},   // FOOD-2
		construction.SunLamp:                {Role: Consumer, Class: ClassLamp, Draw: 2900, Toggle: true},            // FOOD-2
		construction.SentryTurret:           {Role: Consumer, Class: ClassDefense, Draw: 120, Idle: 40, Toggle: true},
		construction.RadioConsole:           {Role: Consumer, Class: ClassBench, Draw: 150, Idle: 15, Toggle: true},
		construction.ElectricSmithy:         {Role: Consumer, Class: ClassBench, Draw: 210, Idle: 21, Toggle: true},
		construction.MachiningTable:         {Role: Consumer, Class: ClassBench, Draw: 350, Idle: 35, Toggle: true},
		construction.FabricationBench:       {Role: Consumer, Class: ClassBench, Draw: 250, Idle: 25, Toggle: true},
		construction.ElectricSmelter:        {Role: Consumer, Class: ClassBench, Draw: 700, Idle: 70, Toggle: true},
		construction.BiofuelRefinery:        {Role: Consumer, Class: ClassBench, Draw: 170, Idle: 17, Toggle: true},
		construction.Autocannon:             {Role: Consumer, Class: ClassDefense, Draw: 150, Idle: 150, Toggle: true},
		construction.UraniumSlugTurret:      {Role: Consumer, Class: ClassDefense, Draw: 150, Idle: 150, Toggle: true},
		construction.Autodoor:               {Role: Consumer, Class: ClassDefense, Draw: 50, Idle: 50, Toggle: true},
	}
	seen := 0
	for _, d := range construction.All() {
		got := SpecOf(d.Kind)
		if got != want[d.Kind] {
			t.Errorf("SpecOf(%s) = %+v, want %+v", d.Name, got, want[d.Kind])
		}
		if _, ok := want[d.Kind]; ok {
			seen++
		}
	}
	if seen != len(want) {
		t.Fatalf("%d of %d electrical kinds are buildable", seen, len(want))
	}
	if s := SpecOf(construction.TorchLamp); s != (Spec{}) {
		t.Errorf("the torch lamp burns logs, not power: %+v", s)
	}
	if s := SpecOf(construction.Kind(250)); s != (Spec{}) {
		t.Errorf("an unknown kind is not electrical: %+v", s)
	}
}

// TestNetOutWh shows a net's energy in whole Wh, as the inspector does.
func TestNetOutWh(t *testing.T) {
	t.Parallel()
	o := NetOut{Stored: 3*WTicksPerWh + WTicksPerWh - 1, Cap: 1200 * WTicksPerWh}
	if o.StoredWh() != 3 || o.CapWh() != 1200 {
		t.Fatalf("StoredWh %d, CapWh %d", o.StoredWh(), o.CapWh())
	}
}
