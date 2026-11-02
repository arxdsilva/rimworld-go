package water

import (
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// WG-08: a huge river has its own width row, wider than a large river,
// with a deep core, one connected channel and no panic on any map size.
func TestHugeRiverWiderThanLarge(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		lc := fixture(seed, 250, 250, riverSite(site.LargeRiver, 6, 2))
		hc := fixture(seed, 250, 250, riverSite(site.HugeRiver, 6, 2))
		run(t, lc, River{})
		run(t, hc, River{})
		lw, hw := channelWidth(lc), channelWidth(hc)
		if hw <= lw*1.15 {
			t.Fatalf("seed %d: huge width %.2f not clearly above large %.2f", seed, hw, lw)
		}
		if deep := count(hc, func(i int) bool { return hc.F.River[i] && hc.F.Water[i] == mapgen.Deep }); deep == 0 {
			t.Fatalf("seed %d: huge river has no deep core", seed)
		}
	}
	for _, dim := range [][2]int{{200, 160}, {60, 60}, {24, 24}} {
		c := fixture(4, dim[0], dim[1], riverSite(site.HugeRiver, 0, 4))
		run(t, c, River{})
		wet := make([]bool, c.W*c.H)
		for i := range wet {
			wet[i] = channel(c, i)
		}
		if _, sizes := mapgen.Components(c.W, c.H, wet, false); len(sizes) != 1 {
			t.Fatalf("%v: %d channel pieces", dim, len(sizes))
		}
	}
}
