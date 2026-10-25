package construction

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/terrain"
)

func TestComponentBuildings(t *testing.T) {
	t.Parallel()
	tc := DefOf(ToolCabinet)
	if tc.Name != "tool cabinet" || tc.Wood != 30 || tc.Work != 120 || tc.Components != 1 || !tc.BlocksMove ||
		tc.Size != [2]int{} || tc.Placed {
		t.Fatalf("tool cabinet %+v", tc)
	}
	if !slices.Equal(Stuffs(ToolCabinet), []item.Kind{item.Steel}) || DefaultStuff(ToolCabinet) != item.Steel ||
		StuffOK(ToolCabinet, item.Wood) || !StuffOK(ToolCabinet, item.Steel) {
		t.Fatal("a tool cabinet is steel only")
	}
	hb := DefOf(HospitalBed)
	if hb.Name != "hospital bed" || hb.Wood != 45 || hb.Work != 200 || hb.Components != 2 || !hb.Rotatable ||
		hb.Size != [2]int{1, 2} || hb.BlocksMove {
		t.Fatalf("hospital bed %+v", hb)
	}
	if !slices.Equal(Stuffs(HospitalBed), []item.Kind{item.Wood, item.Steel}) || DefaultStuff(HospitalBed) != item.Wood {
		t.Fatalf("hospital bed stuffs %v", Stuffs(HospitalBed))
	}
	if !HospitalBed.Sleeps() || HospitalBed.Sleepers() != 1 || ComponentCost(HospitalBed) != 2 || ComponentCost(Bed) != 0 {
		t.Fatal("a hospital bed sleeps one like a bed")
	}
	if cells, _, _ := Footprint(HospitalBed, East, 3, 3); len(cells) != 2 || cells[0][1] != cells[1][1] {
		t.Fatalf("hospital bed on its side %v", cells)
	}
}

func TestPodShellDeconstructsToSteel(t *testing.T) {
	t.Parallel()
	d := DefOf(PodShell)
	if d.Name != "pod shell" || !d.Placed || d.BlocksMove || !PodShell.Instant() || d.Size != [2]int{} {
		t.Fatalf("pod shell %+v", d)
	}
	if Refund(PodShell) != 6 || DefaultStuff(PodShell) != item.Steel || !slices.Equal(Stuffs(PodShell), []item.Kind{item.Steel}) {
		t.Fatalf("pod shell salvages 6 steel: refund %d stuff %v", Refund(PodShell), DefaultStuff(PodShell))
	}
	for _, k := range All() {
		if k.Placed && k.Kind != PodShell && k.Kind != SteamVent && k.Kind != SleeperCasket {
			t.Fatalf("%s is placed-only", k.Name)
		}
	}
}

// MAPGEN-2: a steam vent is a natural 2x2 map feature, walkable and
// never built, salvaged or swapped.
func TestSteamVentIsNatural(t *testing.T) {
	t.Parallel()
	d := DefOf(SteamVent)
	if d.Name != "steam vent" || !d.Placed || !d.Natural || d.BlocksMove || d.Size != [2]int{2, 2} || Refund(SteamVent) != 0 {
		t.Fatalf("steam vent %+v", d)
	}
	for _, k := range All() {
		if k.Natural && k.Kind != SteamVent {
			t.Fatalf("%s is natural", k.Name)
		}
	}
}

func TestComponentFramesWait(t *testing.T) {
	t.Parallel()
	f := Frame{Kind: HospitalBed, Need: 45, Wood: 45, CompNeed: 2, Comp: 1}
	if f.Ready() {
		t.Fatal("a frame waits for its components")
	}
	if f.Comp = 2; !f.Ready() {
		t.Fatal("materials and components in: ready")
	}
	if (Frame{Need: 5, Wood: 5}).Ready() != true || (Frame{Need: 5, Wood: 4}).Ready() {
		t.Fatal("frames without components keep the old rule")
	}
	if (Frame{Need: 30, Wood: 10, CompNeed: 1, Comp: 1}).Ready() {
		t.Fatal("components alone are not enough")
	}
}

func TestSteelFurniture(t *testing.T) {
	t.Parallel()
	for _, k := range []Kind{Table, Chair, Bed, DoubleBed} {
		if !StuffOK(k, item.Steel) || !StuffOK(k, item.Wood) || StuffOK(k, item.GraniteBlocks) {
			t.Errorf("%s: wood or steel, not stone", k)
		}
	}
}

// MAPGEN-3: a bridge is a foundation holding light things; a stone or
// steel wall or a floor needs firm ground.
func TestBridgeSupport(t *testing.T) {
	t.Parallel()
	if d := DefOf(Bridge); !d.Foundation || d.Layer != LayerFoundation || Provides(Bridge) != terrain.SupportLight || Movable(Bridge) {
		t.Fatalf("bridge %+v", d)
	}
	cases := []struct {
		k    Kind
		s    item.Kind
		want terrain.Support
	}{
		{Wall, item.Wood, terrain.SupportLight},
		{Wall, item.GraniteBlocks, terrain.SupportHeavy},
		{Wall, item.Steel, terrain.SupportHeavy},
		{Floor, item.Wood, terrain.SupportHeavy},
		{Carpet, item.CarpetStraw, terrain.SupportHeavy},
		{Bed, item.Wood, terrain.SupportLight},
		{Door, item.Steel, terrain.SupportLight},
		{Bridge, item.Wood, terrain.SupportNone},
	}
	for _, c := range cases {
		if got := Requires(c.k, c.s); got != c.want {
			t.Errorf("Requires(%s, %v) = %d, want %d", DefOf(c.k).Name, c.s, got, c.want)
		}
	}
	if Provides(Wall) != terrain.SupportNone {
		t.Fatal("only foundations provide support")
	}
	if d := DefOf(SleeperCasket); !d.Placed || d.Size != [2]int{1, 2} || Refund(SleeperCasket) != 20 || Movable(SleeperCasket) {
		t.Fatalf("casket %+v", d)
	}
}
