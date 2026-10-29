package ebiten

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-018 HUD-TOP: the colonist bar, top-centre, in the layout's slots.
// Tests that read the canvas trace or change the UI scale never run in
// parallel.

// htApp is a live colony on a stub simulation at the logical size sw x sh
// (the cursor parked off-screen, the game paused).
func htApp(t *testing.T, s *stubSim, sw, sh int) (*App, *fakeInput) {
	t.Helper()
	if s.w == 0 {
		s.w, s.h = 24, 24
	}
	a := testApp(s)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != screenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	a.sw, a.sh = sw, sh
	a.renderer.speed = speedPaused
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	f.Frame(t, a)
	return a, f
}

// htPawns are n alive colonists (ids 1..n) with different moods.
func htPawns(n int) []game.PawnView {
	out := make([]game.PawnView, n)
	for i := range out {
		out[i] = game.PawnView{
			ID: entity.EntityID(i + 1), Name: fmt.Sprintf("Pawn%02d", i+1), X: 2 + i%10, Y: 2 + i/10,
			Status: "alive", Mood: 15 + (i*17)%80, Job: "haul do", Activity: schedule.Work,
		}
	}
	return out
}

// htTrace records every canvas draw while fn runs.
func htTrace(t *testing.T, fn func()) []traceEvent {
	t.Helper()
	var ev []traceEvent
	gfx.SetDrawTrace(func(e traceEvent) { ev = append(ev, e) })
	t.Cleanup(func() { gfx.SetDrawTrace(nil) })
	fn()
	gfx.SetDrawTrace(nil)
	return ev
}

// physRect is r in physical pixels at UI scale s.
func htPhysRect(r rect, s float64) rect {
	k := float32(s)
	return rect{X: r.X * k, Y: r.Y * k, W: r.W * k, H: r.H * k}
}

func htInside(e traceEvent, r rect) bool {
	const eps = 0.51
	return e.X >= r.X-eps && e.Y >= r.Y-eps && e.X+e.W <= r.Right()+eps && e.Y+e.H <= r.Bottom()+eps
}

func TestBarSlotsFollowTheLayout(t *testing.T) {
	for _, n := range []int{6, 30} {
		for _, s := range []float64{1, 1.5, 2} {
			t.Run(fmt.Sprintf("%d colonists at %.1fx", n, s), func(t *testing.T) {
				sw, sh := int(1280/s), int(720/s) // the logical window at this UI scale
				a, _ := htApp(t, &stubSim{pawns: htPawns(n)}, sw, sh)
				l := a.hud()
				if len(l.BarSlots) == 0 {
					t.Fatal("the layout has no bar slots")
				}
				buttons := a.colonistBarButtons(sw, sh)
				if want := n - l.BarMore; len(buttons) != want {
					t.Fatalf("%d bar buttons, want %d (%d colonists, %d hidden)", len(buttons), want, n, l.BarMore)
				}
				for i, b := range buttons {
					slot := l.BarSlots[i]
					if b.X != slot.X || b.Y != slot.Y || b.W != slot.W || b.H != slot.H {
						t.Errorf("button %s = (%v,%v,%v,%v), want slot %+v", b.ID, b.X, b.Y, b.W, b.H, slot)
					}
					if want := barSlotID(entity.EntityID(i + 1)); b.ID != want {
						t.Errorf("button %d id = %q, want %q", i, b.ID, want)
					}
				}
				// the bar stays clear of the resources and the right column
				for i, slot := range l.BarSlots {
					if slot.X < hudMargin+resourcesW+hudMargin || slot.Right() > l.Right.X-hudMargin {
						t.Errorf("slot %d %+v reaches the resources or the right column (right column at %v)", i, slot, l.Right.X)
					}
					if slot.Y < hudMargin {
						t.Errorf("slot %d %+v above the top margin", i, slot)
					}
				}
				// the colonist chrome is the layout's: no legacy band
				assertChromeIsTheLayout(t, a)
			})
		}
	}
}

func TestBarRowsAtTheDefaultSize(t *testing.T) {
	a, _ := htApp(t, &stubSim{pawns: htPawns(6)}, 1280, 720)
	l := a.hud()
	if l.BarScale != 1 || l.BarMore != 0 || len(l.BarSlots) != 6 {
		t.Fatalf("6 colonists: scale %v, hidden %d, slots %d; want one row at scale 1", l.BarScale, l.BarMore, len(l.BarSlots))
	}
	if l.Bar.Y != hudMargin {
		t.Errorf("bar top = %v, want the %d px margin", l.Bar.Y, hudMargin)
	}
	if mid := l.Bar.X + l.Bar.W/2; mid < 639 || mid > 641 {
		t.Errorf("bar centre = %v, want 640 (top-centre)", mid)
	}

	a, _ = htApp(t, &stubSim{pawns: htPawns(30)}, 1280, 720)
	l = a.hud()
	rows := map[float32]int{}
	for _, s := range l.BarSlots {
		rows[s.Y]++
	}
	counts := []int{}
	for _, c := range rows {
		counts = append(counts, c)
	}
	slices.Sort(counts)
	if l.BarScale != 0.75 || l.BarMore != 0 || !slices.Equal(counts, []int{12, 18}) {
		t.Fatalf("30 colonists: scale %v, hidden %d, rows %v; want 18 + 12 at 0.75", l.BarScale, l.BarMore, counts)
	}
}

func TestBarHiddenWhenTheSettingIsOff(t *testing.T) {
	a, f := htApp(t, &stubSim{pawns: htPawns(6)}, 1280, 720)
	a.settings.Gameplay.ShowColonistBar = false
	f.Frame(t, a)
	if n := a.barCount(); n != 0 {
		t.Fatalf("barCount = %d with the bar off, want 0", n)
	}
	if l := a.hud(); !l.Bar.Empty() || len(l.BarSlots) != 0 {
		t.Fatalf("layout bar = %+v with the bar off", l.Bar)
	}
	if bs := a.colonistBarButtons(a.sw, a.sh); len(bs) != 0 {
		t.Fatalf("%d bar buttons with the bar off", len(bs))
	}
	if _, ok := a.uiSt().Find(barMoreID); ok {
		t.Fatal("the +N slot is declared with the bar off")
	}
	ev := htTrace(t, func() { a.drawColonistBar(ebiten.NewImage(1280, 720), 1280, 720) })
	if len(ev) != 0 {
		t.Fatalf("the bar drew %d things while hidden", len(ev))
	}
}

func TestBarLeavesOutPrisoners(t *testing.T) {
	ps := htPawns(4)
	ps[1].Prisoner = true
	a, _ := htApp(t, &stubSim{pawns: ps}, 1280, 720)
	if n := a.barCount(); n != 3 {
		t.Fatalf("barCount = %d, want 3 (the prisoner is not on the bar)", n)
	}
	for _, b := range a.colonistBarButtons(a.sw, a.sh) {
		if b.ID == barSlotID(2) {
			t.Fatal("the prisoner has a bar slot")
		}
	}
}

func TestBarCardDrawsMoodNameAndPortraitBox(t *testing.T) {
	for _, n := range []int{6, 30} {
		for _, s := range []float64{1, 1.5, 2} {
			t.Run(fmt.Sprintf("%d at %.1fx", n, s), func(t *testing.T) {
				sw, sh := int(1280/s), int(720/s)
				ps := htPawns(n)
				a, _ := htApp(t, &stubSim{pawns: ps}, sw, sh)
				withUIScale(t, s) // after the frames: Update re-derives the scale from the settings
				l := a.hud()
				wantNames := l.BarScale >= 1 // smaller slots leave the name to the tooltip
				if n == 6 && s == 1 && !wantNames || n == 30 && s == 1 && wantNames {
					t.Fatalf("setup: bar scale %v for %d colonists", l.BarScale, n)
				}
				screen := ebiten.NewImage(int(float64(sw)*s), int(float64(sh)*s))
				ev := htTrace(t, func() { a.drawColonistBar(screen, sw, sh) })

				names := map[string]bool{}
				for _, e := range ev {
					if e.Kind == "text" {
						names[e.Text] = true
					}
				}
				for i, slot := range l.BarSlots[:barShown(l)] {
					p := ps[i]
					pr := htPhysRect(slot, s)
					// the mood backing: a fill on the slot's floor, as tall as the mood
					gauge := false
					for _, e := range ev {
						if e.Kind == "fill" && e.X >= pr.X-0.5 && e.X+e.W <= pr.Right()+0.5 && e.W > pr.W-1 &&
							abs32(e.Y+e.H-pr.Bottom()) < 0.51 && abs32(e.H-pr.H*float32(p.Mood)/100) < 0.51 {
							gauge = true
						}
					}
					if !gauge {
						t.Errorf("slot %d (%s, mood %d): no mood backing fill of %.1f px on its floor", i, p.Name, p.Mood, pr.H*float32(p.Mood)/100)
					}
					if names[p.Name] != wantNames {
						t.Errorf("slot %d name %q drawn = %v, want %v at bar scale %v", i, p.Name, names[p.Name], wantNames, l.BarScale)
					}
				}
				// nothing the bar draws leaves its bounding box
				box := htPhysRect(l.Bar, s)
				for _, e := range ev {
					if !htInside(e, box) {
						t.Errorf("bar draw %+v outside the bar box %+v", e, box)
					}
				}
			})
		}
	}
}

func TestBarSelectedSlotHasAnOutline(t *testing.T) {
	a, _ := htApp(t, &stubSim{pawns: htPawns(6)}, 1280, 720)
	a.renderer.Select(3)
	l := a.hud()
	screen := ebiten.NewImage(1280, 720)
	ev := htTrace(t, func() { a.drawColonistBar(screen, 1280, 720) })
	outlined := func(slot rect) int {
		n := 0
		for _, e := range ev {
			if e.Kind == "stroke" && e.X == slot.X+1 && e.Y == slot.Y+1 && e.W == slot.W-2 && e.H == slot.H-2 {
				n++
			}
		}
		return n
	}
	for i, slot := range l.BarSlots {
		want := 0
		if i == 2 {
			want = 1
		}
		if got := outlined(slot); got != want {
			t.Errorf("slot %d: %d selection outlines, want %d", i, got, want)
		}
	}
	// a colonist in a group selection is outlined too
	a.renderer.ClearSelect()
	a.setGroup(selPawn, []entity.EntityID{2, 5})
	ev = htTrace(t, func() { a.drawColonistBar(screen, 1280, 720) })
	for i, slot := range l.BarSlots {
		want := 0
		if i == 1 || i == 4 {
			want = 1
		}
		if got := outlined(slot); got != want {
			t.Errorf("group: slot %d: %d outlines, want %d", i, got, want)
		}
	}
}

func TestBarStatusIconsAreDrawnInTheSlot(t *testing.T) {
	ps := htPawns(3)
	ps[0].Drafted = true
	ps[1].MentalBreak, ps[1].Fleeing = "wander", true
	// ps[2] stays plain
	a, _ := htApp(t, &stubSim{pawns: ps}, 1280, 720)
	l := a.hud()
	screen := ebiten.NewImage(1280, 720)
	ev := htTrace(t, func() { a.drawColonistBar(screen, 1280, 720) })
	pips := func(slot rect) int { // no art in the unit tests: each status degrades to a pip
		n := 0
		for _, e := range ev {
			if e.Kind == "circle" && htInside(e, slot) {
				n++
			}
		}
		return n
	}
	for i, want := range []int{1, 2, 0} {
		if got := pips(l.BarSlots[i]); got != want {
			t.Errorf("slot %d draws %d status marks, want %d", i, got, want)
		}
	}
}

func TestBarTooltipNamesTheStatuses(t *testing.T) {
	ps := htPawns(2)
	ps[0].Drafted, ps[0].Lying = true, true
	a, _ := htApp(t, &stubSim{pawns: ps}, 1280, 720)
	bs := a.colonistBarButtons(a.sw, a.sh)
	if want := "Pawn01 (drafted, sleeping)"; bs[0].Tip != want {
		t.Errorf("tip = %q, want %q", bs[0].Tip, want)
	}
	// a plain colonist keeps the legacy line (name, age, title)
	if bs[1].Tip != barTip(ps[1]) || bs[1].Tip != "Pawn02" {
		t.Errorf("plain tip = %q, want %q", bs[1].Tip, barTip(ps[1]))
	}
}

func TestBarBurningFromTheFiresUnderAColonist(t *testing.T) {
	ps := htPawns(2)
	s := &stubSim{pawns: ps, world: game.WorldView{Fires: []game.FireView{{X: ps[1].X, Y: ps[1].Y, Size: 40}}}}
	a, _ := htApp(t, s, 1280, 720)
	if got := a.barStatusesOf(ps[0]); len(got) != 0 {
		t.Errorf("colonist away from the fire: %v", got)
	}
	got := a.barStatusesOf(ps[1])
	if len(got) != 1 || got[0].key != "burning" {
		t.Errorf("colonist in the fire: %v, want burning", got)
	}
}

func TestBarIdleIconWaitsOutShortGaps(t *testing.T) {
	s := &stubSim{pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive", Job: "idle", Activity: schedule.Work}}}
	a, f := htApp(t, s, 1280, 720)
	idle := func() bool {
		for _, st := range a.barStatusesOf(s.pawns[0]) {
			if st.key == "idle" {
				return true
			}
		}
		return false
	}
	if idle() {
		t.Fatal("idle shows the moment the colonist runs out of work")
	}
	s.tick += barIdleHold - 1
	f.Frame(t, a)
	if idle() {
		t.Fatalf("idle shows after %d ticks, before the %d tick hold", barIdleHold-1, barIdleHold)
	}
	s.tick++
	f.Frame(t, a)
	if !idle() {
		t.Fatal("idle does not show after the hold")
	}
	// it goes at once when work starts, and the hold starts over after
	s.pawns[0].Job = "haul do"
	f.Frame(t, a)
	if idle() {
		t.Fatal("idle stays while the colonist works")
	}
	s.pawns[0].Job = "idle"
	s.tick += 5
	f.Frame(t, a)
	if idle() {
		t.Fatal("idle shows at once on the second gap: the hold did not restart")
	}
	// an idle spell from before a new game (the clock is earlier now) does not carry over
	s.pawns[0].Activity = schedule.Work
	s.tick += barIdleHold
	f.Frame(t, a)
	if !idle() {
		t.Fatal("setup: idle should show after the hold")
	}
	s.tick = 5
	f.Frame(t, a)
	if idle() {
		t.Fatal("idle shows from a spell that began before the clock restarted")
	}
	// free time with nothing to do is not worth a flag
	s.pawns[0].Activity = schedule.Recreation
	s.tick += barIdleHold * 2
	f.Frame(t, a)
	if idle() {
		t.Fatal("idle shows during recreation hours")
	}
}

func TestBarMoreSlotCyclesTheHiddenColonists(t *testing.T) {
	a, f := htApp(t, &stubSim{pawns: htPawns(30)}, 700, 720)
	l := a.hud()
	if l.BarMore < 2 {
		t.Fatalf("setup: %d hidden, want an overflowing bar", l.BarMore)
	}
	shown := len(l.BarSlots) - 1 // the last slot is the "+N"
	w, ok := a.uiSt().Find(barMoreID)
	if !ok {
		t.Fatal("no +N widget")
	}
	last := l.BarSlots[len(l.BarSlots)-1]
	if w.R != last {
		t.Errorf("+N widget %+v, want the last slot %+v", w.R, last)
	}
	if want := fmt.Sprintf("%d more colonists", l.BarMore); !strings.HasPrefix(w.Tip, want) {
		t.Errorf("+N tip = %q, want it to start with %q", w.Tip, want)
	}
	click(t, a, barMoreID)
	if !a.renderer.HasSelect() || a.renderer.Selected() != entity.EntityID(shown+1) {
		t.Fatalf("first +N click selected %d (has %v), want the first hidden colonist %d", a.renderer.Selected(), a.renderer.HasSelect(), shown+1)
	}
	f.Frame(t, a)
	click(t, a, barMoreID)
	if a.renderer.Selected() != entity.EntityID(shown+2) {
		t.Fatalf("second +N click selected %d, want the next hidden colonist %d", a.renderer.Selected(), shown+2)
	}
	// the last hidden colonist wraps round to the first
	a.renderer.Select(30)
	click(t, a, barMoreID)
	if a.renderer.Selected() != entity.EntityID(shown+1) {
		t.Fatalf("after the last hidden colonist the click selected %d, want %d", a.renderer.Selected(), shown+1)
	}
}

func TestBarMoreSlotDrawsItsCount(t *testing.T) {
	a, _ := htApp(t, &stubSim{pawns: htPawns(30)}, 700, 720)
	l := a.hud()
	ev := htTrace(t, func() { a.drawColonistBar(ebiten.NewImage(700, 720), 700, 720) })
	want := fmt.Sprintf("+%d", l.BarMore)
	for _, e := range ev {
		if e.Kind == "text" && e.Text == want && htInside(e, l.BarSlots[len(l.BarSlots)-1]) {
			return
		}
	}
	t.Fatalf("no %q text in the last slot", want)
}

func TestBarShiftClickAndHoverStillWork(t *testing.T) {
	a, _ := htApp(t, &stubSim{pawns: htPawns(3)}, 1280, 720)
	click(t, a, barSlotID(2))
	if a.renderer.Selected() != 2 {
		t.Fatalf("bar click selected %d", a.renderer.Selected())
	}
	for _, b := range a.colonistBarButtons(a.sw, a.sh) {
		if b.ID == barSlotID(2) && !b.Selected {
			t.Error("the selected colonist's button is not marked selected")
		}
		if b.ID != barSlotID(2) && b.Selected {
			t.Errorf("%s is marked selected", b.ID)
		}
	}
}

// With the real art every colonist draws a portrait and a drafted one its
// icon, all inside its own slot, at every UI scale.
func TestBarPortraitsAndIconsStayInTheirSlots(t *testing.T) {
	art := testArt(t)
	for _, s := range []float64{1, 1.5, 2} {
		sw, sh := int(1280/s), int(720/s)
		ps := htPawns(6)
		ps[2].Drafted = true
		a, _ := htApp(t, &stubSim{pawns: ps}, sw, sh)
		a.art, a.renderer.art = art, art
		withUIScale(t, s)
		l := a.hud()
		screen := ebiten.NewImage(int(float64(sw)*s), int(float64(sh)*s))
		ev := htTrace(t, func() { a.drawColonistBar(screen, sw, sh) })
		for i, slot := range l.BarSlots {
			pr := htPhysRect(slot, s)
			n, icons := 0, 0
			for _, e := range ev {
				if e.Kind != "image" || e.X+e.W/2 < pr.X || e.X+e.W/2 > pr.Right() || e.Y+e.H/2 < pr.Y || e.Y+e.H/2 > pr.Bottom() {
					continue
				}
				if !htInside(e, pr) {
					t.Errorf("s=%v slot %d: image %v leaves its slot %+v", s, i, e, pr)
				}
				n++
				if abs32(e.W-float32(barIconSize)*l.BarScale*float32(s)) < 0.6 && abs32(e.H-e.W) < 0.6 {
					icons++
				}
			}
			if n == 0 {
				t.Errorf("s=%v slot %d draws no portrait image", s, i)
			}
			if want := map[bool]int{true: 1, false: 0}[i == 2]; icons != want {
				t.Errorf("s=%v slot %d draws %d status icons, want %d", s, i, icons, want)
			}
		}
	}
}
