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
