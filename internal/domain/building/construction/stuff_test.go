package construction

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestStuffStatsGraniteMatchesLegacy(t *testing.T) {
	t.Parallel()
	// Before SIM-029 stone meant HP ×2 and work ×160/100; granite blocks
	// (and the legacy Stone kind, migrated to granite) keep exactly that.
	for _, d := range All() {
		woodHP := MaxHP(d.Kind, item.Wood)
		legacyWork := d.Work * 160 / 100
		for _, s := range []item.Kind{item.GraniteBlocks, item.Stone} {
			if got := MaxHP(d.Kind, s); got != woodHP*2 {
				t.Errorf("%s from %v: HP %d, legacy %d", d.Name, s, got, woodHP*2)
			}
			if got := WorkFor(d.Kind, s); got != legacyWork {
				t.Errorf("%s from %v: work %d, legacy %d", d.Name, s, got, legacyWork)
			}
		}
	}
	if MaxHP(Wall, item.GraniteBlocks) != 320 || WorkFor(Wall, item.GraniteBlocks) != 64 {
		t.Fatalf("granite wall: HP %d work %d", MaxHP(Wall, item.GraniteBlocks), WorkFor(Wall, item.GraniteBlocks))
	}
}

func TestStuffStatsTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s           item.Kind
		hpPct, work int
	}{
		{item.Wood, 100, 100},
		{item.GraniteBlocks, 200, 160},
		{item.SandstoneBlocks, 150, 125},
		{item.LimestoneBlocks, 170, 140},
		{item.SlateBlocks, 160, 135},
		{item.MarbleBlocks, 140, 150},
		{item.Steel, 140, 90},
		{item.GraniteChunk, 100, 100}, // not stuff
		{item.Gold, 100, 100},
	}
	for _, tc := range tests {
		hp, work := StuffStats(tc.s)
		if hp != tc.hpPct || work != tc.work {
			t.Errorf("%v: %d/%d, want %d/%d", tc.s, hp, work, tc.hpPct, tc.work)
		}
		if got := MaxHP(Wall, tc.s); got != 160*tc.hpPct/100 {
			t.Errorf("%v wall HP %d", tc.s, got)
		}
		if got := WorkFor(Wall, tc.s); got != 40*tc.work/100 {
			t.Errorf("%v wall work %d", tc.s, got)
		}
	}
}

func TestStuffsLists(t *testing.T) {
	t.Parallel()
	blocks := item.StoneBlocks()
	withBlocks := append([]item.Kind{item.Wood}, blocks...)
	tests := []struct {
		k    Kind
		want []item.Kind
	}{
		{Wall, append(slices.Clone(withBlocks), item.Steel)},
		{Floor, withBlocks},
		{Door, []item.Kind{item.Wood, item.Steel}},
		{Table, []item.Kind{item.Wood}},
	}
	for _, tc := range tests {
		got := Stuffs(tc.k)
		if !slices.Equal(got, tc.want) {
			t.Errorf("Stuffs(%s) = %v, want %v", DefOf(tc.k).Name, got, tc.want)
		}
		if got[0] != DefaultStuff(tc.k) {
			t.Errorf("%s: default %v is not first", DefOf(tc.k).Name, DefaultStuff(tc.k))
		}
		for _, s := range got {
			if !StuffOK(tc.k, s) {
				t.Errorf("StuffOK(%s, %v) = false but listed", DefOf(tc.k).Name, s)
			}
		}
		for _, s := range []item.Kind{item.Stone, item.GraniteChunk, item.Silver, item.Gold} {
			if StuffOK(tc.k, s) {
				t.Errorf("StuffOK(%s, %v) = true", DefOf(tc.k).Name, s)
			}
		}
	}
	if StuffOK(Floor, item.Steel) || StuffOK(Table, item.MarbleBlocks) || StuffOK(Door, item.SlateBlocks) {
		t.Fatal("material accepted where it is not listed")
	}
}

func TestStonecutterIsBench(t *testing.T) {
	t.Parallel()
	d := DefOf(StonecutterTable)
	if d.Kind != StonecutterTable || d.Name != "stonecutter table" || !d.Bench || !d.BlocksMove || !d.Rotatable || !d.HasSpot {
		t.Fatalf("stonecutter def = %+v", d)
	}
	if d.Size != [2]int{2, 1} || d.Spot != [2]int{0, 1} || d.Layer != LayerObject || d.Wood != 6 || d.Work != 160 {
		t.Fatalf("stonecutter shape = %+v", d)
	}
	if !slices.Equal(Stuffs(StonecutterTable), []item.Kind{item.Wood}) {
		t.Fatalf("stonecutter stuffs = %v", Stuffs(StonecutterTable))
	}
}
