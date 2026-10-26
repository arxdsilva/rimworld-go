package game

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-021 contract: the electricity skeleton (kinds, commands, phase,
// incidents, save field) is wired but inert until its agents fill it, so
// colonies play exactly as before.

// TestPowerSkeletonInert: the power commands, the phase and the flick
// hooks change nothing, the power events never fit, and a save carries
// no power state.
func TestPowerSkeletonInert(t *testing.T) {
	t.Parallel()
	a := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	b := NewColony(Config{Seed: 3, Width: 48, Height: 40})
	target := uint64(1) // whatever it is: the stubs ignore it
	if len(a.bldgIDs) > 0 {
		target = uint64(a.bldgIDs[0])
	}
	a.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: target})
	a.Enqueue(command.Command{Kind: command.SetThermostat, Target: target, Count: 180})
	a.Step(120)
	b.Step(120)
	if a.StateHash() != b.StateHash() || a.pw != nil || a.exportPower() != nil {
		t.Fatal("the power skeleton changed the game")
	}
	now := uint64(a.clock.Now())
	for _, id := range []incident.ID{incident.SolarFlare, incident.ShortCircuit} {
		if a.incidentFits(id, now) {
			t.Fatalf("%s fits before EVENTS implements it", incident.DefOf(id).Key)
		}
	}
	p := a.pawnIDs[0]
	if a.assignFlick(p) {
		t.Fatal("no flick to assign")
	}
	if _, why := a.flickJob(p, 1, 1); why != ReasonGone {
		t.Fatalf("forced flick: %q", why)
	}
}

// TestConduitLayerPlaces: a conduit (LayerConduit) is placed, completed
// and found on its own layer, under the layer index's reach.
func TestConduitLayerPlaces(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	bid := buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 4, 4)
	if g.layerBldgAt(4, 4, construction.LayerObject) != 0 || g.layerBldgAt(4, 4, construction.LayerConduit) != bid {
		t.Fatal("the conduit stands on its own layer")
	}
}

// TestGlowView: a lamp glows only on power (PLAY-021 CONSUMERS: re-expressed
// from "lamps always glow"), a torch lamp and a campfire only while
// burning; nothing else does, and a colony without devices has no nets.
func TestGlowView(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 10})
	lamp := buildNow(t, g, construction.Lamp, item.Wood, construction.South, 2, 2)
	fire := buildNow(t, g, construction.Campfire, item.Wood, construction.South, 8, 2)
	chair := buildNow(t, g, construction.Chair, item.Wood, construction.South, 14, 2)
	torch := buildNow(t, g, construction.TorchLamp, item.Wood, construction.South, 17, 2)
	view := func(id entity.EntityID) BuildingView {
		for _, bv := range g.World().Buildings {
			if bv.ID == id {
				return bv
			}
		}
		t.Fatalf("no view of %d", id)
		return BuildingView{}
	}
	if v := view(lamp); v.Glow {
		t.Fatal("a lamp without power gives no light")
	}
	if v := view(fire); v.Glow || v.Burning {
		t.Fatal("an unfuelled campfire gives no light")
	}
	g.setFuel(fire, 1000)
	if v := view(fire); !v.Glow || !v.Burning || v.GlowRadius != campfireRadius {
		t.Fatalf("burning campfire: glow %v radius %d", v.Glow, v.GlowRadius)
	}
	if v := view(torch); v.Glow {
		t.Fatal("an unfuelled torch lamp gives no light")
	}
	g.setFuel(torch, 1000)
	if v := view(torch); !v.Glow || v.GlowRadius != torchRadius {
		t.Fatalf("burning torch lamp: glow %v radius %d", v.Glow, v.GlowRadius)
	}
	v := view(chair)
	if v.Glow || v.PowerRole != "" || v.Powered || v.HasLink || v.PowerNet != 0 {
		t.Fatalf("chair: %+v", v)
	}
	if w := g.World(); w.PowerNets != nil || w.SolarFlare || w.PowerVer != 0 {
		t.Fatal("no power nets yet")
	}
	// on power the lamp glows: a conduit beside it and a fuelled generator
	buildNow(t, g, construction.PowerConduit, item.Steel, construction.South, 3, 2)
	gen := buildNow(t, g, construction.WoodGenerator, item.Steel, construction.South, 4, 3)
	g.setFuel(gen, g.fuelCap(gen))
	g.Step(powerInterval)
	if v := view(lamp); !v.Glow || !v.Powered || v.GlowRadius != lampRadius {
		t.Fatalf("powered lamp: glow %v powered %v radius %d", v.Glow, v.Powered, v.GlowRadius)
	}
}

// TestPowerSaveGob: every powerSave field survives the save encoding.
func TestPowerSaveGob(t *testing.T) {
	t.Parallel()
	in := saveState{Power: &powerSave{
		Charge: []entry[int64]{{ID: 4, V: 900 * 2500}}, WantOff: []entity.EntityID{5}, IsOff: []entity.EntityID{6},
		Unpowered: []entity.EntityID{7}, Shed: []entry[uint64]{{ID: 8, V: 12500}},
		Thermo: []entry[thermoSave]{{ID: 9, V: thermoSave{Target: 210, Active: true}}}, FlareUntil: 30000, Last: 29950,
	}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&in); err != nil {
		t.Fatal(err)
	}
	var out saveState
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Power, out.Power) {
		t.Fatalf("power save %+v, want %+v", out.Power, in.Power)
	}
}
