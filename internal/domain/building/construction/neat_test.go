package construction

import "testing"

// TestNeatStorageKinds (NST-A): the 27 kinds are in the doc's order with the
// published sizes, 18 rotate, none blocks movement, and each has hit points,
// a beauty row and a profile number.
func TestNeatStorageKinds(t *testing.T) {
	t.Parallel()
	type row struct {
		k    Kind
		name string
		w, h int
		rot  bool
	}
	rows := []row{
		{DisplayCase, "display case", 1, 1, true}, {RoundPedestal, "round pedestal", 1, 1, false},
		{SmallDisplayShelf, "small display shelf", 1, 1, true}, {DisplayShelf, "display shelf", 2, 1, true},
		{WeaponRack, "weapon rack", 2, 1, true}, {NarrowWardrobe, "narrow wardrobe", 1, 1, true},
		{Wardrobe, "wardrobe", 2, 1, true}, {NarrowDisplayCabinet, "narrow display cabinet", 1, 1, true},
		{DisplayCabinet, "display cabinet", 2, 1, true}, {OpenShelf, "open shelf", 2, 1, true},
		{TwoTierDisplayShelf, "two tier display shelf", 2, 1, true}, {NarrowTallShelf, "narrow tall shelf", 1, 1, true},
		{TallShelf, "tall shelf", 2, 1, true}, {Safe, "safe", 1, 1, false}, {Drum, "drum", 1, 1, false},
		{HorizontalWeaponRack, "horizontal weapon rack", 2, 1, true}, {VerticalWeaponRack, "vertical weapon rack", 2, 1, true},
		{TextileRack, "textile rack", 1, 1, true}, {MeatHook, "meat hook", 2, 1, true},
		{MiniPallet, "mini pallet", 1, 1, false}, {HalfPallet, "half pallet", 2, 1, true}, {Pallet, "pallet", 2, 2, false},
		{SmallCrate, "small crate", 1, 1, false}, {Crate, "crate", 2, 1, true}, {LargeCrate, "large crate", 2, 2, false},
		{TallCrate, "tall crate", 2, 2, false}, {CorpseDumpSpot, "corpse dump spot", 1, 1, false},
	}
	if len(rows) != 27 {
		t.Fatalf("%d rows, %d kinds", len(rows), kindCount)
	}
	rot, cells := 0, 0
	names := map[string]bool{}
	for i, r := range rows {
		d := DefOf(r.k)
		if r.k != DisplayCase+Kind(i) || d.Name != r.name || names[r.name] {
			t.Errorf("%s: order or name (%q)", r.name, d.Name)
		}
		names[r.name] = true
		w, h := d.Size[0], d.Size[1]
		if w == 0 {
			w, h = 1, 1
		}
		if w != r.w || h != r.h || d.Rotatable != r.rot {
			t.Errorf("%s: size %dx%d rotatable %v", r.name, w, h, d.Rotatable)
		}
		cells += w * h
		if d.Rotatable {
			rot++
		}
		p, ok := NeatProfile(r.k)
		if d.BlocksMove || d.Wood <= 0 || d.Work <= 0 || !r.k.IsNeatStorage() || !ok || p != i+1 {
			t.Errorf("%s: blocks %v wood %d work %d profile %d", r.name, d.BlocksMove, d.Wood, d.Work, p)
		}
		if hp, ok := axMaxHP(r.k); !ok || hp < 50 {
			t.Errorf("%s: hit points %d", r.name, hp)
		}
		if r.k != Safe && r.k != SmallCrate && r.k != Crate && furniture[r.k].Beauty == 0 {
			t.Errorf("%s: no beauty", r.name)
		}
		if len(Stuffs(r.k)) == 0 || !StuffOK(r.k, DefaultStuff(r.k)) {
			t.Errorf("%s: stuffs", r.name)
		}
	}
	if rot != 18 || cells != 48 {
		t.Errorf("rotatable %d, cells %d", rot, cells)
	}
	if _, ok := NeatProfile(FishTank); ok || FishTank.IsNeatStorage() || (lastNeat + 1).IsNeatStorage() {
		t.Error("neighbours are not neat storage")
	}
}
