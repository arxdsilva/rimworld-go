// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

const sendID = "modal." + FormCaravanID + ".form.send"

// openForm opens the Form caravan dialog over two colonists, two live
// prisoners (one downed) and a dead one, ten simple meals and some wood,
// a tame animal and one site, and runs two frames.
func openForm(t *testing.T) (*fakeShell, *Dealings) {
	t.Helper()
	s, dl := newFake()
	s.pawns = []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive"},
		{ID: 2, Name: "Bo", Status: "alive"},
		{ID: 3, Name: "Cy", Status: "downed", Prisoner: true},
		{ID: 4, Name: "Di", Status: "alive", Prisoner: true},
		{ID: 5, Name: "Ed", Status: "dead", Prisoner: true},
	}
	s.w.Items = []game.ItemView{{ID: 20, Kind: item.Meal.String(), Qty: 10}, {ID: 21, Kind: item.Wood.String(), Qty: 50}}
	s.w.Animals = []game.AnimalView{{ID: 9, Species: "dustbuck", Tame: true}}
	s.sites = []game.WorldSiteView{{Name: "Ridge camp", Faction: "Ridge", Relation: "friendly", Tile: 7, Visitable: true}}
	dl.OpenFormCaravan(func() int { return -1 })
	s.frame(dl)
	s.frame(dl)
	return s, dl
}

// pick presses widget id with the mouse (a check box changes its draft in
// the frame of the press) and runs a frame after it.
func pick(t *testing.T, s *fakeShell, dl *Dealings, id string) {
	t.Helper()
	w, ok := s.kit.Find(id)
	if !ok {
		t.Fatalf("no widget %s", id)
	}
	s.cur = [2]int{int(w.R.X + w.R.W/2), int(w.R.Y + w.R.H/2)}
	var raw inputframe.Raw
	raw.Press[inputframe.Left] = true
	s.f = inputframe.NewFrame(raw)
	s.frame(dl)
	s.f, s.cur = inputframe.NewFrame(inputframe.Raw{}), [2]int{-1, -1}
	s.frame(dl)
}

// TestCaravanListsPrisonersApart: prisoners appear only under Prisoners,
// a downed one is disabled with its reason, the dead are not listed, and
// with none the colonist rows stand alone.
func TestCaravanListsPrisonersApart(t *testing.T) {
	t.Parallel()
	s, _ := openForm(t)
	for _, id := range []string{"form.member.1", "form.member.2", "form.prisoner.3", "form.prisoner.4"} {
		if _, ok := s.kit.Find(id); !ok {
			t.Errorf("no widget %s", id)
		}
	}
	for _, id := range []string{"form.member.3", "form.member.4", "form.prisoner.1", "form.prisoner.5", "form.member.5"} {
		if _, ok := s.kit.Find(id); ok {
			t.Errorf("widget %s should not be listed", id)
		}
	}
	if w, _ := s.kit.Find("form.prisoner.3"); !w.Disabled || w.Reason != "Down: cannot travel." {
		t.Errorf("a downed prisoner: %+v", w)
	}
	if w, _ := s.kit.Find("form.prisoner.4"); w.Disabled {
		t.Errorf("a prisoner who can walk is disabled: %+v", w)
	}

	none, dl := newFake()
	none.pawns = []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}}
	dl.OpenFormCaravan(func() int { return -1 })
	none.frame(dl)
	none.frame(dl)
	if _, ok := none.kit.Find("form.member.1"); !ok {
		t.Error("no colonist row without prisoners")
	}
}

// TestCaravanNameOrder: Name is the colonists, the prisoners, the
// animals, then the stacks; a lone colonist stays "1".
func TestCaravanNameOrder(t *testing.T) {
	t.Parallel()
	s, dl := openForm(t)
	pick(t, s, dl, "form.dest.7")
	pick(t, s, dl, "form.member.1")
	pick(t, s, dl, sendID)
	if len(s.cmds) != 1 || s.cmds[0].Kind != command.FormCaravan || s.cmds[0].Name != "1" || s.cmds[0].Count != 7 {
		t.Fatalf("a lone colonist: %+v", s.cmds)
	}

	s, dl = openForm(t)
	pick(t, s, dl, "form.dest.7")
	pick(t, s, dl, "form.member.2")
	pick(t, s, dl, "form.member.1")
	pick(t, s, dl, "form.prisoner.4")
	pick(t, s, dl, "form.animal.9")
	pick(t, s, dl, "form.goods."+item.Meal.String()+".inc")
	pick(t, s, dl, "form.goods."+item.Meal.String()+".inc")
	pick(t, s, dl, sendID)
	if len(s.cmds) != 1 || s.cmds[0].Name != "1,2,4,9,20:2" {
		t.Fatalf("name order: %+v", s.cmds)
	}

	s, dl = openForm(t)
	pick(t, s, dl, "form.dest.7")
	pick(t, s, dl, "form.prisoner.4")
	if w, _ := s.kit.Find(sendID); !w.Disabled || w.Reason != FormNoMembers {
		t.Fatalf("a caravan of prisoners alone: %+v", w)
	}
}

// TestCaravanCapCountsPrisoners: a prisoner carries what a colonist does
// (the simulation's Pack and Pace count every person alike).
func TestCaravanCapCountsPrisoners(t *testing.T) {
	t.Parallel()
	s, dl := openForm(t)
	f := &formCaravan{picked: map[entity.EntityID]bool{1: true}, captives: map[entity.EntityID]bool{4: true}, beasts: map[entity.EntityID]bool{9: true}}
	if got, want := f.capKg(dl), 2*caravan.PersonCarryKg+packKg("dustbuck"); got != want {
		t.Fatalf("cap %d kg, want %d", got, want)
	}
	f.captives = nil
	if got, want := f.capKg(dl), caravan.PersonCarryKg+packKg("dustbuck"); got != want {
		t.Fatalf("cap without the prisoner %d kg, want %d", got, want)
	}
	_ = s
}

// TestCaravanFoodAgainstTravel: the summary weighs the picked food's days
// against the simulation's travel estimate; a shortfall warns but leaves
// Send enabled, and an estimate that fails shows its reason.
func TestCaravanFoodAgainstTravel(t *testing.T) {
	t.Parallel()
	s, dl := openForm(t)
	meal := item.Meal.String()
	f := &formCaravan{picked: map[entity.EntityID]bool{1: true}, captives: map[entity.EntityID]bool{4: true},
		beasts: map[entity.EntityID]bool{}, goods: map[string]int{meal: 10}, dest: 7}
	food := float64(caravan.Days(10*item.Meal.Nutrition(), 2)) / 1000
	if got := fmt.Sprintf("%.1f", food); got != "2.8" {
		t.Fatalf("10 simple meals for 2 eaters feed %s days, want 2.8", got)
	}
	wantFood := fmt.Sprintf("Food: %.1f days", food)

	s.est.ticks = 2 * clock.TicksPerDay // travel 2.0 days: the food covers it
	head, line, short := f.summaryLines(dl)
	if line != wantFood+" · travel: 2.0 days" || short || !strings.HasPrefix(head, "1 colonist(s), 1 prisoner(s) · ") {
		t.Fatalf("enough food: %q / %q short=%v", head, line, short)
	}
	s.est.ticks = 5 * clock.TicksPerDay
	f.dest = 8 // another road: asked again
	if _, line, short = f.summaryLines(dl); line != wantFood+" · travel: 5.0 days" || !short {
		t.Fatalf("short of food: %q short=%v", line, short)
	}
	// the warning is display only: Send stays enabled
	pick(t, s, dl, "form.dest.7")
	pick(t, s, dl, "form.member.1")
	if w, _ := s.kit.Find(sendID); w.Disabled {
		t.Fatalf("a food warning must not disable Send: %+v", w)
	}

	s.est.ticks, s.est.why = 0, "No way to get there."
	f.dest = 9
	if _, line, short = f.summaryLines(dl); line != wantFood+" · travel: No way to get there." || short {
		t.Fatalf("a failing estimate: %q short=%v", line, short)
	}
	s.est.why = "no such reason"
	f.dest = 10
	if _, line, _ = f.summaryLines(dl); line != wantFood+" · travel: no such reason" {
		t.Fatalf("an unknown reason shows as given: %q", line)
	}

	f.dest = -1 // no destination: the food alone
	if _, line, short = f.summaryLines(dl); line != wantFood || short {
		t.Fatalf("no destination: %q", line)
	}
	f.dest, f.picked, f.captives = 7, nil, nil // no eaters
	if _, line, _ = f.summaryLines(dl); line != "Food: 0.0 days" {
		t.Fatalf("no eaters: %q", line)
	}
}

// TestCaravanPlanReasonsAreTranslated: the simulation's estimate reasons
// pass through the translator (the catalog holds each).
func TestCaravanPlanReasonsAreTranslated(t *testing.T) {
	t.Parallel()
	s, dl := newFake()
	s.mark = true // T marks what it translates
	for _, why := range []string{"No world map.", "No way to get there.", "Too slow to get there.", "Pick a destination for the caravan."} {
		if got := planWhy(dl, why); got != "~"+why {
			t.Errorf("planWhy(%q) = %q, want it translated", why, got)
		}
	}
	if got := planWhy(dl, "something else"); got != "something else" {
		t.Errorf("an unknown reason shows as given: %q", got)
	}
}

// TestCaravanEstimateMemoized: unchanged picks ask the simulation once,
// however many frames are drawn; changing the eaters asks again.
func TestCaravanEstimateMemoized(t *testing.T) {
	t.Parallel()
	s, dl := openForm(t)
	pick(t, s, dl, "form.dest.7")
	pick(t, s, dl, "form.member.1")
	s.est.calls = 0
	for range 10 {
		s.frame(dl)
	}
	if s.est.calls != 0 {
		t.Fatalf("%d estimates over 10 unchanged frames, want 0 (already asked)", s.est.calls)
	}
	pick(t, s, dl, "form.prisoner.4")
	for range 10 {
		s.frame(dl)
	}
	if s.est.calls != 1 {
		t.Fatalf("%d estimates after one change, want 1", s.est.calls)
	}
}

// TestCaravanFoodGroup: edible goods sit under Food, the rest after them.
func TestCaravanFoodGroup(t *testing.T) {
	t.Parallel()
	food, other := splitFood(dealingsGoods(t))
	if len(food) != 1 || food[0].kind != item.Meal.String() || len(other) != 2 {
		t.Fatalf("food %v other %v", food, other)
	}
}

func dealingsGoods(t *testing.T) []goodsRow {
	t.Helper()
	s, dl := openForm(t)
	s.w.Items = append(s.w.Items, game.ItemView{ID: 22, Kind: item.Hay.String(), Qty: 3})
	return dl.goodsRows()
}

// TestCaravanShowsSickMembers: the caravan row notes its sick and hurt
// members, and its hover card gives every member's health (WS-M).
func TestCaravanShowsSickMembers(t *testing.T) {
	t.Parallel()
	c := game.CaravanView{Members: []string{"Ada", "Bo", "Cy"}, Health: []string{"flu", "", "bleeding"}}
	if n := CaravanHealthNote(c); n != " · Ada: flu · Cy: bleeding" {
		t.Fatalf("note %q", n)
	}
	tip := CaravanTip(c)
	for _, want := range []string{"Caravan: Ada, Bo, Cy", "Ada: flu", "Bo: well", "Cy: bleeding"} {
		if !strings.Contains(tip, want) {
			t.Fatalf("tip %q lacks %q", tip, want)
		}
	}
	if n := CaravanHealthNote(game.CaravanView{Members: []string{"Ada"}}); n != "" {
		t.Fatalf("a well caravan notes %q", n)
	}
}
