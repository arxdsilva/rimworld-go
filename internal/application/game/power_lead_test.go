package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// PLAY-021 lead follow-ups: a wood-fired generator runs (and burns fuel)
// only while switched on and outside a solar flare; a pending flick shows
// in the order menu; a device's view names both ends of its link line.

func TestGeneratorRunsOnlyWhenOnAndNotFlared(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	gen := psFuelled(t, g, 5, 5)
	if !g.generatorRunning(gen) {
		t.Fatal("a fuelled generator that is on runs")
	}
	g.setFlare(uint64(g.Tick()) + 1000)
	if g.generatorRunning(gen) {
		t.Fatal("a generator runs during a solar flare")
	}

	g = New(Config{Seed: 1, Width: 30, Height: 20})
	gen = psFuelled(t, g, 5, 5)
	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(gen), Enabled: false})
	g.Step(1)
	g.flick(gen)
	if g.powerOn(gen) || g.generatorRunning(gen) {
		t.Fatal("a generator switched off runs")
	}
}

func TestOrdersOfferAPendingFlick(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	sw := buildNow(t, g, construction.PowerSwitch, item.Steel, construction.South, 9, 4)
	ada := psWorker(t, g, 5, 8)
	has := func() bool {
		for _, o := range g.OrdersAt(ada, 9, 4) {
			if strings.HasPrefix(o.Label, "Flick ") && o.Cmd.Order == command.OrderFlick {
				return true
			}
		}
		return false
	}
	if has() {
		t.Fatal("a flick offered with nothing pending")
	}
	g.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(sw), Enabled: false})
	g.Step(1)
	if !has() {
		t.Fatal("no Flick order for the pending switch")
	}
}

func TestDeviceViewNamesBothLinkEnds(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	psConduits(t, g, 4, 10, 16)
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 17, 5)
	psFuelled(t, g, 8, 3)
	g.Step(powerInterval)
	v := psView(t, g, lamp)
	if !v.HasLink || v.LinkFromX != 17 || v.LinkFromY != 5 || v.LinkX != 16 || v.LinkY != 4 {
		t.Fatalf("lamp link from %d,%d to %d,%d (has %v), want 17,5 to 16,4", v.LinkFromX, v.LinkFromY, v.LinkX, v.LinkY, v.HasLink)
	}
}

// TestBurnerAndPowerViewFields: every fuel burner (campfire, torch lamp,
// wood-fired generator) reports its fuel in the view, a producer its peak
// output, and a heater whether its thermostat is working now.
func TestBurnerAndPowerViewFields(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	gen := psFuelled(t, g, 5, 5)
	torch := buildNow(t, g, construction.TorchLamp, item.Wood, construction.South, 12, 5)
	g.setFuel(torch, g.fuelCap(torch)/2)
	heater := buildNow(t, g, construction.Heater, item.Steel, construction.South, 15, 5)
	g.setThermoActive(heater, true)
	g.Step(powerInterval)
	// the running generator has burned a little of its full tank
	if v := psView(t, g, gen); !v.Fuel || v.FuelPct < 95 || v.FuelPct > 100 || v.PowerMax != 800 {
		t.Fatalf("generator view fuel %v %d%%, max %d W", v.Fuel, v.FuelPct, v.PowerMax)
	}
	if v := psView(t, g, torch); !v.Fuel || v.FuelPct < 45 || v.FuelPct > 50 { // burning too
		t.Fatalf("torch view fuel %v %d%%", v.Fuel, v.FuelPct)
	}
	if v := psView(t, g, heater); !v.ThermoActive {
		t.Fatal("an active heater's view says it is not working")
	}
}
