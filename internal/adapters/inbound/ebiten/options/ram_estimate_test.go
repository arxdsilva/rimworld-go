package options

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/sysinfo"
)

const gib = sysinfo.GiB

// ---- the model (pure) ------------------------------------------------------

func TestEstimatedRAMGrowsWithTheMap(t *testing.T) {
	t.Parallel()
	if got := EstimatedRAM(250) - EstimatedRAM(0); got != 250*250*RamPerTile {
		t.Fatalf("250 adds %d bytes over the base, want side^2 x %d", got, RamPerTile)
	}
	if EstimatedRAM(0) != RamBase {
		t.Fatalf("EstimatedRAM(0) = %d, want the base %d", EstimatedRAM(0), RamBase)
	}
	prev := EstimatedRAM(0)
	for _, s := range []int{120, 200, 250, 300, 400, 500} {
		if e := EstimatedRAM(s); e <= prev {
			t.Errorf("EstimatedRAM(%d) = %d is not above the smaller map's %d", s, e, prev)
		} else {
			prev = e
		}
	}
	if EstimatedRAM(-5) != RamBase {
		t.Error("a negative side counts as no tiles")
	}
	// The measured footprint fit: 1850 MiB at no tiles, and the 250 ->
	// grown 500 difference (1180 MiB over 187 500 tiles) per tile.
	if RamBase != 1850<<20 || RamPerTile != 6600 {
		t.Errorf("base %d, per tile %d; want 1850 MiB and 6600 B (measured peak footprint)", RamBase, RamPerTile)
	}
	if got := EstimatedRAM(500) >> 20; got < 3400 || got > 3450 {
		t.Errorf("grown 500 estimate %d MiB, want the measured 3422 MiB", got)
	}
	if RamPerTile < 256 {
		t.Errorf("per-tile cost %d is below the 256-byte floor", RamPerTile)
	}
}

func TestMinimumRAMIsOneAndAHalfPlusTwoGiBRoundedUp(t *testing.T) {
	t.Parallel()
	need := EstimatedRAM(DefaultMapSide)*3/2 + 2*gib
	got := MinimumRAM()
	if got%gib != 0 {
		t.Errorf("minimum %d is not a whole number of GiB", got)
	}
	if got < need || got >= need+gib {
		t.Errorf("minimum %d is not %d rounded up to a GiB", got, need)
	}
}

func TestGBRounding(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		b           uint64
		ceil, round int
	}{
		{0, 0, 0},
		{1, 1, 0},
		{gib, 1, 1},
		{gib + 1, 2, 1},
		{gib * 3 / 2, 2, 2},        // 1.5 GiB rounds half up
		{gib * 37 / 10, 4, 4},      // 3.7
		{gib * 79 / 10, 8, 8},      // a "8 GB" PC that reports 7.9 GiB
		{16 * gib, 16, 16},         // exact
		{16*gib - 100<<20, 16, 16}, // 15.9 GiB
	} {
		if got := GBCeil(c.b); got != c.ceil {
			t.Errorf("GBCeil(%d) = %d, want %d", c.b, got, c.ceil)
		}
		if got := GBRound(c.b); got != c.round {
			t.Errorf("GBRound(%d) = %d, want %d", c.b, got, c.round)
		}
	}
}

func TestLowRAMTextNamesBothAmounts(t *testing.T) {
	t.Parallel()
	want := fmt.Sprintf("This computer has 4 GB of memory; the game needs about %d GB to run smoothly. It will use low graphics settings.",
		MinimumRAM()/gib)
	if got := LowRAMText(4 * gib); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
	// Just under the minimum must never read "has 6 GB; needs about 6 GB".
	just := MinimumRAM() - 1
	if !strings.Contains(LowRAMText(just), fmt.Sprintf("has %d GB", MinimumRAM()/gib-1)) {
		t.Errorf("a machine just under the minimum reads %q", LowRAMText(just))
	}
}

func TestMapSizeNoteOnlyOverSeventyPercentOfRAM(t *testing.T) {
	t.Parallel()
	// A machine whose 70% line falls between a 350 and a 400 map.
	total := EstimatedRAM(375) * 10 / 7
	for _, s := range []int{120, 200, 250, 300, 350} {
		if n := MapSizeNote(s, total); n != "" {
			t.Errorf("%d fits (%d of %d bytes) but warns %q", s, EstimatedRAM(s), total, n)
		}
	}
	for _, s := range []int{400, 450, 500} {
		want := fmt.Sprintf("Needs about %d GB; this computer has %d GB.", GBCeil(EstimatedRAM(s)), GBRound(total))
		if n := MapSizeNote(s, total); n != want {
			t.Errorf("%d: note %q, want %q", s, n, want)
		}
	}
	// The line itself: exactly 70% does not warn, a byte less memory does.
	est := EstimatedRAM(300)
	if MapSizeNote(300, (est*10+6)/7) != "" {
		t.Error("an estimate at or under 70% of RAM must not warn")
	}
	if MapSizeNote(300, est*10/7-1) == "" {
		t.Error("an estimate over 70% of RAM must warn")
	}
}

func TestMapSizeNoteWithUnknownHardware(t *testing.T) {
	t.Parallel()
	for _, s := range []int{120, 200, 250, 300, 349} {
		if n := MapSizeNote(s, 0); n != "" {
			t.Errorf("%d with unknown hardware: %q, want nothing", s, n)
		}
	}
	for _, s := range []int{350, 400, 500} {
		if n := MapSizeNote(s, 0); n != "Large map: needs a capable computer" {
			t.Errorf("%d with unknown hardware: %q, want the gentle large-map note", s, n)
		}
	}
	// The fallback line is the application's "Large" flag.
	for _, o := range game.MapSizes() {
		if o.Large != (o.Side >= LargeMapSide) {
			t.Errorf("%d: game says Large=%v but the UI fallback starts at %d", o.Side, o.Large, LargeMapSide)
		}
	}
}
