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

// TestTempContractToggleVentInert (TEMP-0, live since TEMP-1): ToggleVent
// aimed at anything but a vent is ignored, so a colony that gets it hashes
// as before and keeps in step with a twin that did not; the room model
// runs (TempVer moves; thermo_wiki_test.go proves the vent itself).
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
		t.Fatal("ToggleVent at a non-vent changed the state")
	}
	g.Step(300)
	twin.Step(300)
	if g.StateHash() != twin.StateHash() {
		t.Fatal("the commanded colony drifted from its twin")
	}
	w := g.World()
	if w.ClimateEvent != "" || w.ClimateEventEnds != 0 || w.TempVer == 0 {
		t.Fatalf("climate view: event %q ends %d, TempVer %d (the room model runs)", w.ClimateEvent, w.ClimateEventEnds, w.TempVer)
	}
	for _, b := range w.Buildings {
		if b.Open {
			t.Fatalf("%s reads open: only a vent does", b.Kind)
		}
	}
}

// TestTempContractBuildings (TEMP-0): the passive cooler and the vent are
// valid kinds; the vent fills a wall gap like the cooler (it bounds rooms,
// is placed in a wall, faces across it and is never moved), and neither
// draws power (TEMP-1: the vent is a toggle off the grid, flicked open or
// shut like a power switch).
func TestTempContractBuildings(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.PassiveCooler, construction.Vent} {
		d := construction.DefOf(k)
		if got, ok := construction.Parse(d.Name); !ok || got != k || d.Wood <= 0 || d.Work <= 0 || !d.BlocksMove ||
			construction.MaxHP(k, item.Wood) <= 0 {
			t.Errorf("%s: %+v", k, d)
		}
		if s := power.SpecOf(k); s.Role != power.None || s.Draw != 0 || s.Idle != 0 || s.Toggle != (k == construction.Vent) {
			t.Errorf("%s: power spec %+v", k, s)
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

// TestTempContractGarments (TEMP-0, live since TEMP-4): the seven
// garments parse, file as apparel and stack one to a tile, and are worn
// in their layer slot, stored, made, valued (wiki) and given by a
// scenario; outfits hold all eight slots.
func TestTempContractGarments(t *testing.T) {
	t.Parallel()
	slot := map[item.Kind]item.Slot{item.TShirt: item.SkinTorso, item.ButtonDownShirt: item.SkinTorso, item.Pants: item.SkinLegs,
		item.Jacket: item.Outer, item.CowboyHat: item.Head, item.BowlerHat: item.Head, item.Tribalwear: item.SkinTorso}
	value := map[item.Kind]int{item.TShirt: 66, item.ButtonDownShirt: 77, item.Pants: 66, item.Jacket: 130, item.CowboyHat: 44,
		item.BowlerHat: 37, item.Tribalwear: 97}
	for k, s := range slot {
		if got, ok := item.ParseKind(k.String()); !ok || got != k || k.Category() != "Apparel" || k.StackLimit() != 1 {
			t.Errorf("%s: parse %v %v, category %q, stack %d", k, got, ok, k.Category(), k.StackLimit())
		}
		if !k.IsApparel() || k.Slot() != s || k.Value() != value[k] || !zone.Storable(k) ||
			!slices.Contains(item.Apparel(), k) || !slices.Contains(scenario.ItemChoices(), k) {
			t.Errorf("%s: slot %s value %d, not live", k, k.Slot(), k.Value())
		}
		if !slices.ContainsFunc(recipe.All(), func(r recipe.Def) bool { return r.Output == k && r.Bench == construction.TailoringBench }) {
			t.Errorf("no tailoring recipe makes %s", k)
		}
	}
	if item.NumSlots != 8 || item.Outer != item.Body || craft.Slots != 8 {
		t.Fatalf("slots: %d (outer %s), outfit %d; Forced is a uint8", item.NumSlots, item.Outer, craft.Slots)
	}
	if o := item.Parka.Occupies(); !o.Has(item.Outer) || o.Has(item.Middle) || item.ShieldBelt.Occupies() != 1<<item.Belt {
		t.Errorf("Occupies: parka %b, shield belt %b", o, item.ShieldBelt.Occupies())
	}
	if o := item.Tribalwear.Occupies(); !o.Has(item.SkinTorso) || !o.Has(item.SkinLegs) || item.MarineArmor.Occupies().Has(item.Head) ||
		!item.MarineArmor.Occupies().Has(item.Middle) || item.FlakVest.Slot() != item.Middle {
		t.Errorf("Occupies: tribalwear %b, marine armor %b", o, item.MarineArmor.Occupies())
	}
}

// TestTempContractConditionsMoodsIncidents (TEMP-0): the cold conditions,
// the temperature ladders and the climate events are defined with their
// wiki numbers, and nothing fires the events yet.
func TestTempContractConditionsMoodsIncidents(t *testing.T) {
	t.Parallel()
	for _, k := range []body.CondKind{body.CondHypothermia, body.CondHypothermicSlowdown, body.CondHeatstroke} { // TEMP-3: frostbite is a wound
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
		if d.Key != key || d.Name == "" || d.Category != incident.Nature || d.Weight <= 0 || d.Cooldown <= 0 {
			t.Errorf("%s: %+v", key, d)
		}
		if _, on := g.incidents.ClimateView(now); on != 0 { // TEMP-2: climate_events_test.go fires them
			t.Errorf("%s under way in a new colony", key)
		}
	}
}
