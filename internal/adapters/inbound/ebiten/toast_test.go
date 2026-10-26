package ebiten

import (
	"fmt"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// PLAY-018: toasts sit at the hint anchor (the layout's HintX / HintBottom,
// just above the command buttons or the pane), newest at the bottom.

func TestToastsStackAtTheHintAnchor(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("x%v", s), func(t *testing.T) {
			a, f := colonyApp(t, messageStub(0))
			pw, ph := rightAtScale(t, a, f, s)
			a.toast("Screenshot saved")
			a.toast("Autosaved")
			a.toast("Not implemented yet")
			l := a.hud()
			rs := a.toastRects(a.sw, a.sh)
			if len(rs) != 3 {
				t.Fatalf("%d toast rects, want 3", len(rs))
			}
			// newest first: it sits at the anchor, the older ones stack above it
			for i, r := range rs {
				if r.X != l.HintX {
					t.Errorf("toast %d x = %v, want HintX %v", i, r.X, l.HintX)
				}
				if r.bottom() > l.HintBottom {
					t.Errorf("toast %d ends at %v, below HintBottom %v", i, r.bottom(), l.HintBottom)
				}
				if r.X < 0 || r.right() > float32(a.sw) || r.Y < 0 {
					t.Errorf("toast %d %+v off screen", i, r)
				}
				if i > 0 && rs[i-1].Y <= r.bottom() {
					t.Errorf("toast %d %+v overlaps the newer one %+v", i, r, rs[i-1])
				}
			}
			if rs[0].bottom() != l.HintBottom {
				t.Errorf("the newest toast ends at %v, want HintBottom %v", rs[0].bottom(), l.HintBottom)
			}
			a.drawToasts(ebiten.NewImage(pw, ph), a.sw, a.sh)
		})
	}
}

// A disabled-reason hint owns the anchor; the toasts stack above it.
func TestToastsMakeRoomForTheHint(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, messageStub(0))
	a.toast("Autosaved")
	free := a.toastRects(a.sw, a.sh)[0]
	a.hint = "Needs a bed"
	held := a.toastRects(a.sw, a.sh)[0]
	if held.bottom() >= free.bottom() || held.bottom() > a.hud().HintBottom-lineStep(tsBody) {
		t.Errorf("with a hint the toast ends at %v (free: %v)", held.bottom(), free.bottom())
	}
}

// Off the colony (menus) toasts keep a plain bottom-centre spot, and they
// expire.
func TestToastsOutsideTheColonyAndExpiry(t *testing.T) {
	t.Parallel()
	a := testApp(&stubSim{w: 4, h: 4})
	a.finishBoot()
	a.toast("Screenshot saved")
	rs := a.toastRects(a.sw, a.sh)
	if len(rs) != 1 {
		t.Fatalf("toasts %v", rs)
	}
	sw, sh := a.size()
	if cx := rs[0].X + rs[0].W/2; cx < float32(sw)/2-1 || cx > float32(sw)/2+1 || rs[0].bottom() > float32(sh) {
		t.Errorf("menu toast %+v is not bottom-centre of %dx%d", rs[0], sw, sh)
	}
	a.updates += toastFrames
	if len(a.liveToasts()) != 0 || len(a.toastRects(sw, sh)) != 0 {
		t.Error("a toast outlives toastFrames")
	}
}
