package ebiten

import "testing"

func TestCropPickerListsTrees(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.zones")
	click(t, a, "arch.tool.grow")
	b, ok := btnByID(a.architectButtons(), "arch.crop.tree")
	if !ok || b.disabled {
		t.Fatalf("tree entry missing or locked: %+v ok=%v", b, ok)
	}
	if td, _ := toolByID("grow"); len(td.desc) < 21 || td.desc[:21] != "Pick a crop or trees " {
		t.Fatalf("desc = %q", td.desc)
	}
	// picking Tree sends the tree species through the zone command
	click(t, a, "arch.crop.tree")
	a.archDrag = true
	a.archX0, a.archY0, a.archX1, a.archY1 = 6, 6, 8, 7
	a.submitArchitect()
	g.Step(1)
	z := g.World().GrowZones
	if len(z) != 1 || z[0].Crop != "tree" {
		t.Fatalf("zones = %+v", z)
	}
}
