package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/command"
)

// TestTempContractToggleVentInert (TEMP-0): ToggleVent routes to a no-op,
// so a colony that gets it hashes as before and keeps in step with a twin
// that did not; the temperature view fields stay zero.
func TestTempContractToggleVentInert(t *testing.T) {
	t.Parallel()
	cfg := Config{Seed: 7, Width: 40, Height: 30}
	g, twin := NewColony(cfg), NewColony(cfg)
	g.Step(200)
	twin.Step(200)
	before := g.StateHash()
	var target uint64
	if w := g.World(); len(w.Buildings) > 0 {
		target = uint64(w.Buildings[0].ID)
	}
	g.Enqueue(command.Command{Kind: command.ToggleVent, Target: target, Enabled: true})
	g.ApplyPending()
	if g.StateHash() != before {
		t.Fatal("ToggleVent changed the state before TEMP-1")
	}
	g.Step(300)
	twin.Step(300)
	if g.StateHash() != twin.StateHash() {
		t.Fatal("the commanded colony drifted from its twin")
	}
	w := g.World()
	if w.ClimateEvent != "" || w.ClimateEventEnds != 0 || w.TempVer != 0 {
		t.Fatalf("climate view filled before TEMP-1/TEMP-2: %q %d %d", w.ClimateEvent, w.ClimateEventEnds, w.TempVer)
	}
	for _, r := range w.Rooms {
		if r.ExposedWalls != 0 || r.DoubleWalls != 0 || r.HeatRate != 0 || r.RoofKind != "" {
			t.Fatalf("room %d thermal fields filled before TEMP-1: %+v", r.ID, r)
		}
	}
	for _, b := range w.Buildings {
		if b.Open || b.HeatRate != 0 {
			t.Fatalf("%s: vent or heat fields filled before TEMP-1", b.Kind)
		}
	}
	for _, a := range w.Animals {
		if a.ComfortMin != 0 || a.ComfortMax != 0 || a.Hypothermia != 0 || a.Heatstroke != 0 {
			t.Fatalf("%s: animal temperature filled before TEMP-3", a.Label)
		}
	}
	for _, p := range g.Snapshot() {
		if p.Frostbite != 0 {
			t.Fatalf("%s: frostbite before TEMP-3", p.Name)
		}
	}
}

// TestTempContractBuildings (TEMP-0): the passive cooler and the vent are
// valid kinds; the vent fills a wall gap like the cooler (it bounds rooms,
// is placed in a wall, faces across it and is never moved), and neither
// draws power.
func TestTempContractBuildings(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.PassiveCooler, construction.Vent} {
		d := construction.DefOf(k)
		if got, ok := construction.Parse(d.Name); !ok || got != k || d.Wood <= 0 || d.Work <= 0 || !d.BlocksMove ||
			construction.MaxHP(k, item.Wood) <= 0 {
			t.Errorf("%s: %+v", k, d)
		}
		if power.SpecOf(k) != (power.Spec{}) {
			t.Errorf("%s draws power", k)
		}
	}
	v := construction.DefOf(construction.Vent)
	if !construction.Vent.WallMounted() || !construction.Cooler.WallMounted() || !roomWall(construction.Vent) ||
		!autoOriented(construction.Vent) || construction.Movable(construction.Vent) || !v.MetalOnly || !v.Rotatable {
		t.Errorf("vent is not wall-mounted like the cooler: %+v", v)
	}
	if construction.PassiveCooler.WallMounted() || roomWall(construction.PassiveCooler) || construction.DefOf(construction.PassiveCooler).MetalOK {
		t.Error("the passive cooler is a free-standing wooden piece")
	}
}

// TestTempContractGarments (TEMP-0): the seven garments parse, file as
// apparel in the resource list and stack one to a tile, but are not worn,
// stored, made, traded, valued or given by a scenario before TEMP-4; the
// layer slots exist beside the four in use, and outfits keep five slots.
func TestTempContractGarments(t *testing.T) {
	t.Parallel()
	garments := []item.Kind{item.TShirt, item.ButtonDownShirt, item.Pants, item.Jacket, item.CowboyHat, item.BowlerHat,
		item.Tribalwear}
	for _, k := range garments {
		if got, ok := item.ParseKind(k.String()); !ok || got != k || !k.IsContractApparel() || k.Category() != "Apparel" ||
			k.StackLimit() != 1 {
			t.Errorf("%s: parse %v %v, category %q, stack %d", k, got, ok, k.Category(), k.StackLimit())
		}
		if k.IsApparel() || k.Slot() != item.NoSlot || k.Occupies() != 0 || k.Value() != 0 || zone.Storable(k) ||
			slices.Contains(item.Apparel(), k) || slices.Contains(scenario.ItemChoices(), k) {
			t.Errorf("%s is live before TEMP-4", k)
		}
	}
	for _, r := range recipe.All() {
		if r.Output.IsContractApparel() {
			t.Errorf("recipe %s makes %s before TEMP-4", r.Name, r.Output)
		}
	}
	if item.Parka.IsContractApparel() || item.Rice.IsContractApparel() {
		t.Error("IsContractApparel takes only the seven garments")
	}
	if item.NumSlots != 8 || item.Outer != item.Body || craft.Slots != 5 || int(item.NumSlots) > 8 {
		t.Fatalf("slots: %d (outer %s), outfit %d; Forced is a uint8", item.NumSlots, item.Outer, craft.Slots)
	}
	for _, s := range []item.Slot{item.SkinTorso, item.SkinLegs, item.Middle} {
		if s.String() == "" || int(s) < craft.Slots {
			t.Errorf("slot %d: %q, in an outfit before TEMP-4", s, s)
		}
		for _, k := range item.Apparel() {
			if k.Slot() == s {
				t.Errorf("%s worn in %s before TEMP-4", k, s)
			}
		}
	}
	if o := item.Parka.Occupies(); !o.Has(item.Outer) || o.Has(item.Middle) || item.ShieldBelt.Occupies() != 1<<item.Belt {
		t.Errorf("Occupies: parka %b, shield belt %b", o, item.ShieldBelt.Occupies())
	}
}

// TestTempContractConditionsMoodsIncidents (TEMP-0): the cold conditions,
// the temperature ladders and the climate events are defined with their
// wiki numbers, and nothing fires the events yet.
func TestTempContractConditionsMoodsIncidents(t *testing.T) {
	t.Parallel()
	for _, k := range []body.CondKind{body.CondFrostbite, body.CondHypothermicSlowdown} {
		if d := body.DefOfCond(k); d.Label == "" || d.Lethal || len(d.Stages) == 0 {
			t.Errorf("condition %d: %+v", k, d)
		}
	}
	if st := body.DefOfCond(body.CondHypothermicSlowdown).Stages; len(st) != 5 || st[3].At != 350 || st[3].PainAdd != 150 {
		t.Errorf("hypothermic slowdown stages %+v", st)
	}
	for k, v := range map[mood.Kind]int{mood.Chilly: -4, mood.Shivering: -8, mood.NumbingCold: -12, mood.AbsolutelyFreezing: -16,
		mood.Sweaty: -4, mood.Overheated: -8, mood.Baking: -12, mood.BlisteringHot: -16, mood.Naked: -6} {
		if d := mood.DefOf(k); d.Value != v || d.Label == "" || d.Duration != 0 {
			t.Errorf("mood %d: %+v, want %d", k, d, v)
		}
	}
	if mood.DefOf(mood.Chilly).Group == 0 || mood.DefOf(mood.Chilly).Group != mood.DefOf(mood.AbsolutelyFreezing).Group ||
		mood.DefOf(mood.Sweaty).Group == mood.DefOf(mood.Chilly).Group {
		t.Error("each ladder is one group (its rungs never stack)")
	}
	g := NewColony(Config{Seed: 3, Width: 30, Height: 30})
	now := uint64(g.clock.Now())
	for id, key := range map[incident.ID]string{incident.HeatWave: "heat_wave", incident.ColdSnap: "cold_snap",
		incident.VolcanicWinter: "volcanic_winter"} {
		d := incident.DefOf(id)
		if d.Key != key || d.Name == "" || d.Category != incident.Nature || d.Weight != 0 || d.WeightAt(100) != 0 || d.Cooldown <= 0 {
			t.Errorf("%s: %+v", key, d)
		}
		if g.incidentFits(id, now) || g.FireIncident(key) {
			t.Errorf("%s fires before TEMP-2", key)
		}
	}
}
