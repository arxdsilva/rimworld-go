package hud

import (
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
)

func TestClockText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		h, m int
		h12  bool
		want string
	}{
		{0, 5, false, "00:05"}, {13, 30, false, "13:30"},
		{0, 5, true, "12:05 AM"}, {9, 0, true, "9:00 AM"}, {12, 0, true, "12:00 PM"}, {23, 59, true, "11:59 PM"},
	} {
		if got := ClockText(c.h, c.m, c.h12); got != c.want {
			t.Errorf("ClockText(%d,%d,%v) = %q, want %q", c.h, c.m, c.h12, got, c.want)
		}
	}
}

func TestHourLineRealtime(t *testing.T) {
	t.Parallel()
	d := clock.Date{Hour: 8, Minute: 12}
	wall := time.Date(2026, 1, 1, 14, 7, 0, 0, time.UTC)
	if got := hourLine(d, wall, false, false, "work"); got != "08:12  "+d.Period().String()+" · work" {
		t.Errorf("plain line = %q", got)
	}
	if got := hourLine(d, wall, true, false, "work"); got != "08:12 (14:07)  "+d.Period().String()+" · work" {
		t.Errorf("realtime line = %q", got)
	}
	if got := hourLine(d, wall, true, true, "work"); got != "8:12 AM (2:07 PM)  "+d.Period().String()+" · work" {
		t.Errorf("12h realtime line = %q", got)
	}
	if got := hourLine(d, time.Time{}, true, false, "work"); got != "08:12  "+d.Period().String()+" · work" {
		t.Errorf("no wall clock = %q", got)
	}
	if w := gfx.TextW(hourLine(d, wall, true, true, "work"), gfx.Body); w > GlobalInnerW {
		t.Logf("long line is %v px; Global ellipsizes it to %v", w, GlobalInnerW)
	}
}

func TestShowWeapon(t *testing.T) {
	t.Parallel()
	armed := game.PawnView{Weapon: "rifle"}
	drafted := game.PawnView{Weapon: "rifle", Drafted: true}
	bare := game.PawnView{Weapon: "fists", Drafted: true}
	for _, c := range []struct {
		mode string
		p    game.PawnView
		want bool
	}{
		{config.BarWeaponsNever, drafted, false},
		{config.BarWeaponsDrafted, armed, false},
		{config.BarWeaponsDrafted, drafted, true},
		{config.BarWeaponsAlways, armed, true},
		{config.BarWeaponsAlways, bare, false},
	} {
		if got := showWeapon(c.mode, c.p); got != c.want {
			t.Errorf("showWeapon(%s, %+v) = %v", c.mode, c.p, got)
		}
	}
}

// The weapon icon sits inside the slot and the portrait at every UI scale.
func TestWeaponRectFitsSlot(t *testing.T) {
	t.Parallel()
	for _, ui := range []float32{0.75, 1, 1.25, 1.5, 2} {
		for _, bs := range []float32{0.5, 0.75, 1} {
			slot := gfx.Rect{X: 10, Y: 4, W: 48 * ui * bs, H: 60 * ui * bs}
			card := barCardOf(slot, bs*ui)
			r := weaponRect(card)
			if r.W <= 0 || r.X < card.art.X || r.Y < card.art.Y || r.Right() > card.art.Right()+0.01 || r.Bottom() > card.art.Bottom()+0.01 {
				t.Errorf("ui %v bar %v: icon %+v outside portrait %+v", ui, bs, r, card.art)
			}
			if r.Right() > slot.Right() || r.Bottom() > slot.Bottom() {
				t.Errorf("ui %v bar %v: icon %+v outside slot %+v", ui, bs, r, slot)
			}
		}
	}
}
