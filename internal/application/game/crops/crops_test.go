package crops

import "testing"

// TestCatalogue: every crop has a name, a positive grow time and yield;
// the zone list is the crops then the trees, which yield wood.
func TestCatalogue(t *testing.T) {
	all, zone := All(), ZoneSpecies()
	if len(all) == 0 || len(zone) != len(all)+3 {
		t.Fatalf("%d crops, %d zone species", len(all), len(zone))
	}
	for _, c := range all {
		if c.Name == "" || c.Days <= 0 || c.Yield <= 0 || c.RichDays > c.Days || c.GravelDay < c.Days {
			t.Errorf("crop %+v", c)
		}
	}
	for _, c := range zone[len(all):] {
		if c.Label != "wood" || c.Yield <= 0 {
			t.Errorf("tree %+v", c)
		}
	}
}

// TestIDs: a crop name maps to its species value, a tree only as a zone
// species, an unknown name to nothing.
func TestIDs(t *testing.T) {
	c := All()[0].Name
	if _, ok := ID(c); !ok {
		t.Fatalf("crop %q has no id", c)
	}
	if _, ok := ZoneID(c); !ok {
		t.Fatalf("crop %q is no zone species", c)
	}
	tree := ZoneSpecies()[len(All())].Name
	if _, ok := ID(tree); ok {
		t.Fatalf("tree %q is no crop", tree)
	}
	if _, ok := ZoneID(tree); !ok {
		t.Fatalf("tree %q is a zone species", tree)
	}
	if _, ok := ID("nope"); ok {
		t.Fatal("unknown name")
	}
	if _, ok := ZoneID("nope"); ok {
		t.Fatal("unknown zone name")
	}
}
