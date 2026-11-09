package light

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// TestStatsCountsFlushReasons (GAR-161): every batch that goes out is
// counted under the reason it went, with the calls, batched quads, sheet
// copies and resets of the frame; StartFrame zeroes them.
func TestStatsCountsFlushReasons(t *testing.T) {
	r := groundRig()
	a, big := ebiten.NewImage(8, 8), ebiten.NewImage(24, 24)
	dst1, dst2 := ebiten.NewImage(64, 64), ebiten.NewImage(64, 64)
	r.lit.StartFrame()
	r.lit.Batch(true)
	r.drawGround(dst1, a, 0)
	r.drawGround(dst2, a, 0)   // dst
	r.drawGround(dst2, big, 0) // page
	r.lit.Draw(&r.view, dst2, a, ebiten.GeoM{}, noCS, 0, 0, false, 0.5)
	r.drawGround(dst2, a, 0)
	r.lit.DrawGround(&r.view, dst2, a, ebiten.GeoM{}, noCS, 0, 0, true, 0) // bake
	r.drawGround(dst2, a, 0)
	r.lit.Batch(false) // end
	st := r.lit.Stats()
	want := map[FlushReason]int{FlushDst: 1, FlushPage: 1, FlushBlood: 1, FlushBake: 1, FlushEnd: 1}
	for why := range NumFlushReasons {
		if st.Flushes[why] != want[why] {
			t.Errorf("flush %v: %d, want %d", why, st.Flushes[why], want[why])
		}
	}
	if st.Batched != 5 || st.Calls != 6 || st.Copies != 2 || st.Resets != 0 || st.Calls != r.lit.Calls {
		t.Fatalf("stats %v (Lit.Calls %d); want 5 batched, 6 calls, 2 copies", st, r.lit.Calls)
	}
	if s := st.String(); s != "calls 6 batched 5 copies 2 resets 0 flush dst=1 page=1 blood=1 bake=1 end=1" {
		t.Fatalf("String: %q", s)
	}
	r.lit.StartFrame()
	if r.lit.Stats() != (Stats{}) || r.lit.Calls != 6 {
		t.Fatal("StartFrame zeroes the frame's stats and keeps Calls")
	}
}
