// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// TestJourneyCaravanPrisonerAndFood (#14): the Form caravan dialog lists a
// prisoner apart from the colonists and the people's food apart from other
// goods; ticking a colonist, the prisoner, all the food and a settlement
// and pressing Send emits FormCaravan whose Name holds the colonist, the
// prisoner, then the food as stack:qty. The fake mouse goes through the real
// Update and Draw over the world-tab stub (globeTabSim) with a prisoner
// pawn view and food items: a wizard colony cannot be made to hold a
// prisoner through public APIs in a journey's time, so the simulation's side
// (the prisoner boards, the food rides in the cargo) is proved by
// world_caravan_food_test.go in the game package.
func TestJourneyCaravanPrisonerAndFood(t *testing.T) {
	t.Parallel()
	s := worldStub()
	s.pawns = append(s.pawns, game.PawnView{ID: 3, Name: "Di", Status: "alive", Prisoner: true})
	s.world.Items = []game.ItemView{{ID: 50, Kind: item.Meal.String(), Qty: 10}, {ID: 51, Kind: item.Wood.String(), Qty: 30}}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.deal.OpenFormCaravan(func() int { return -1 })
	f.Frame(t, a)
	send := "modal." + dialogs.FormCaravanID + ".form.send"

	if !hasWidget(a, "form.prisoner.3") || hasWidget(a, "form.member.3") {
		t.Fatal("the prisoner is not listed apart from the colonists")
	}
	if !hasWidget(a, "form.goods."+item.Meal.String()+".all") || !hasWidget(a, "form.goods."+item.Wood.String()+".all") {
		t.Fatal("the food and the other goods have no rows")
	}
	press(t, a, f, "form.prisoner.3")
	f.Frame(t, a)
	if w, _ := a.uiSt().Find(send); !w.Disabled || w.Reason != dialogs.FormNoMembers {
		t.Fatalf("Send with only a prisoner: %+v", w)
	}
	press(t, a, f, "form.member.1")
	press(t, a, f, "form.goods."+item.Meal.String()+".all")
	press(t, a, f, "form.dest.9")
	f.Frame(t, a)
	if w, _ := a.uiSt().Find(send); w.Disabled {
		t.Fatalf("Send with a colonist, a prisoner, food and a destination: %+v", w)
	}
	press(t, a, f, send)
	if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.FormCaravan || s.cmds[0].Count != 9 {
		t.Fatalf("commands %+v, want one FormCaravan to 9", s.cmds)
	}
	if name := s.cmds[0].Name; name != "1,3,50:10" || strings.Contains(name, "51") {
		t.Fatalf("Name %q, want the colonist, the prisoner, then the food only", name)
	}
}
