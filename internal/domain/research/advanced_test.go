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
