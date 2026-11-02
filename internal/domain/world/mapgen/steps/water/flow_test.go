package water

import (
	"testing"

	"rimworld-go/internal/domain/world/site"
)

// AX-4: the river step marks every channel cell with its downstream flow
// (a watermill's wheel turns in it) and draws nothing on a river-less map.
func TestRiverFlow(t *testing.T) {
	c := fixture(5, 150, 150, riverSite(site.River, 0, 4)) // north edge to south edge
	run(t, c, River{})
	if c.R.River == nil {
		t.Fatal("no flow on a river map")
	}
	south := 0
	for i, d := range c.R.River {
		if (d != 0) != channel(c, i) {
			t.Fatalf("cell %d: flow %d, channel %v", i, d, channel(c, i))
		}
		if d >= 4 && d <= 6 { // SE, S, SW
			south++
		}
	}
	if n := count(c, func(i int) bool { return channel(c, i) }); south*2 < n {
		t.Fatalf("a north-to-south river flows south: %d of %d cells", south, n)
	}
	dry := fixture(1, 90, 70, site.Site{Hills: site.Flat})
	run(t, dry, River{})
	if dry.R.River != nil {
		t.Fatal("flow without a river")
	}
	if flowDir(0, -1) != 1 || flowDir(1, 0) != 3 || flowDir(0, 1) != 5 || flowDir(-1, 0) != 7 {
		t.Fatal("flow directions: 1 north, 3 east, 5 south, 7 west")
	}
}
