package research

import "testing"

func TestFabricationAndPharmacology(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		p    Project
		name string
		cost int
		req  Project
	}{{Fabrication, "fabrication", 5000, Gunsmithing}, {Pharmacology, "pharmacology", 4000, Herbalism}} {
		d := DefOf(c.p)
		if d.Project != c.p || d.Name != c.name || d.Cost != c.cost || len(d.Requires) != 1 || d.Requires[0] != c.req ||
			d.Label == "" || d.Desc == "" || len(d.Unlocks) == 0 {
			t.Fatalf("%s = %+v", c.name, d)
		}
		if p, ok := Parse(c.name); !ok || p != c.p {
			t.Fatalf("parse %s", c.name)
		}
	}
	if DefOf(NumProjects).Name != "" {
		t.Fatal("unknown project")
	}
}

func TestProgressSliceAndV1RoundTrip(t *testing.T) {
	t.Parallel()
	var pr Progress
	pr.Add(Traps, 300)
	pr.Add(Pharmacology, 1200)
	if got := FromSlice(pr.Slice()); got != pr {
		t.Fatalf("slice round trip %v", got)
	}
	if got := FromSlice([]int{5, 6}); got[FoodPreservation] != 5 || got[Herbalism] != 6 || got[Fabrication] != 0 {
		t.Fatalf("short slice %v", got)
	}
	if got := FromSlice(make([]int, NumProjects+4)); got != (Progress{}) {
		t.Fatalf("long slice %v", got)
	}
	v1 := pr.V1()
	if v1[Traps] != 300 || len(v1) != V1Projects {
		t.Fatalf("v1 %v", v1)
	}
	if back := FromV1(v1); back[Traps] != 300 || back[Pharmacology] != 0 {
		t.Fatalf("from v1 %v", back)
	}
}

// PLAY-021: Electricity first, the four follow-ups each needing it, and
// every power building gated (the lamp joins with the torch lamp).
func TestElectricityTree(t *testing.T) {
	t.Parallel()
	if d := DefOf(Electricity); d.Cost != 3000 || len(d.Requires) != 0 || d.Name != "electricity" {
		t.Fatalf("electricity %+v", d)
	}
	for p, c := range map[Project]int{Batteries: 2500, SolarPower: 4000, WindPower: 3500, GeothermalPower: 5000} {
		d := DefOf(p)
		if d.Cost != c || len(d.Requires) != 1 || d.Requires[0] != Electricity || len(d.Unlocks) != 1 || d.Label == "" || d.Desc == "" {
			t.Errorf("%s = %+v", d.Name, d)
		}
	}
	var pr Progress
	for key, by := range map[string]Project{"build:power conduit": Electricity, "build:wood-fired generator": Electricity,
		"build:battery": Batteries, "build:solar panel": SolarPower, "build:wind turbine": WindPower,
		"build:geothermal generator": GeothermalPower} {
		if p, ok := pr.Locks(key); !ok || p != by {
			t.Errorf("%s locked by %v %v, want %v", key, p, ok, by)
		}
	}
	if pr.Available(Batteries) || !pr.Available(Electricity) {
		t.Fatal("batteries need electricity first")
	}
	if _, ok := pr.Locks("build:torch lamp"); ok {
		t.Fatal("the torch lamp is free from the start")
	}
}
