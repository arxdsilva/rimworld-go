// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package caravan

import (
	"math"
	"math/rand"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
)

func TestEdibleExcludesHay(t *testing.T) {
	for _, k := range []item.Kind{item.Meal, item.Berries, item.Pemmican} {
		if !Edible(k) {
			t.Errorf("%v should be edible", k)
		}
	}
	for _, k := range []item.Kind{item.Hay, item.Steel, item.Kind(250)} {
		if Edible(k) {
			t.Errorf("%v should not be edible", k)
		}
	}
}

func TestNutritionSumsEdibleOnly(t *testing.T) {
	goods := []item.Stack{
		{Kind: item.Meal, Qty: 2},
		{Kind: item.Berries, Qty: 10},
		{Kind: item.Hay, Qty: 50},
		{Kind: item.Steel, Qty: 9},
		{Kind: item.Meal, Qty: -3},
		{Kind: item.Pemmican, Qty: 0},
	}
	want := 2*item.Meal.Nutrition() + 10*item.Berries.Nutrition()
	if got := Nutrition(goods); got != want {
		t.Fatalf("Nutrition = %d, want %d", got, want)
	}
	if Nutrition(nil) != 0 {
		t.Fatal("nil goods feed nothing")
	}
}

func TestHourNeed(t *testing.T) {
	if HourNeed(0) != 0 || HourNeed(-2) != 0 {
		t.Fatal("no eaters eat nothing")
	}
	if got := HourNeed(3); got != 3*needs.HungerPerHour {
		t.Fatalf("HourNeed(3) = %d", got)
	}
}

func TestDaysMatchesHourlyRate(t *testing.T) {
	for n := 1; n <= 5; n++ {
		if got := Days(HourNeed(n)*24, n); got != 1000 {
			t.Errorf("Days for %d eaters = %d, want 1000", n, got)
		}
	}
	if Days(100, 0) != 0 || Days(100, -1) != 0 || Days(0, 3) != 0 || Days(-5, 3) != 0 {
		t.Fatal("zero cases must be 0")
	}
	if got := Days(1<<50, 1); got != math.MaxInt32 {
		t.Fatalf("large input = %d, want clamp", got)
	}
	if got := Days(math.MaxInt, 1); got != math.MaxInt32 {
		t.Fatalf("huge input = %d, want clamp", got)
	}
}

func TestRationSpoilingFirst(t *testing.T) {
	goods := []item.Stack{
		{Kind: item.SurvivalMeal, Qty: 5},
		{Kind: item.Pemmican, Qty: 100},
		{Kind: item.Meal, Qty: 5},
	}
	unit := item.Meal.Nutrition()
	eat, fed, ok := Ration(goods, 3*unit)
	if !ok || fed != 0 || !reflect.DeepEqual(eat, []item.Stack{{Kind: item.Meal, Qty: 3}}) {
		t.Fatalf("meals first: %v fed %d ok %v", eat, fed, ok)
	}
	eat, _, ok = Ration(goods, 5*unit+item.Pemmican.Nutrition()*2)
	want := []item.Stack{{Kind: item.Meal, Qty: 5}, {Kind: item.Pemmican, Qty: 2}}
	if !ok || !reflect.DeepEqual(eat, want) {
		t.Fatalf("pemmican second: %v", eat)
	}
	eat, _, ok = Ration([]item.Stack{{Kind: item.Meal, Qty: 1}, {Kind: item.Pemmican, Qty: 1}, {Kind: item.SurvivalMeal, Qty: 2}},
		unit+item.Pemmican.Nutrition()+unit)
	if !ok || len(eat) != 3 || eat[0].Kind != item.Meal || eat[1].Kind != item.Pemmican || eat[2].Kind != item.SurvivalMeal {
		t.Fatalf("survival meal last: %v", eat)
	}
}

func TestRationShortAndCredit(t *testing.T) {
	goods := []item.Stack{{Kind: item.Meal, Qty: 1}, {Kind: item.Berries, Qty: 2}, {Kind: item.Hay, Qty: 99}, {Kind: item.Steel, Qty: 4}}
	eat, fed, ok := Ration(goods, 1<<20)
	if ok || fed != 0 {
		t.Fatalf("short: fed %d ok %v", fed, ok)
	}
	if len(eat) != 2 || eat[0].Kind != item.Meal || eat[1].Kind != item.Berries || eat[1].Qty != 2 {
		t.Fatalf("short eats everything edible: %v", eat)
	}
	if eat, fed, ok := Ration(goods, 0); eat != nil || fed != 0 || !ok {
		t.Fatalf("need 0: %v %d %v", eat, fed, ok)
	}
	if eat, fed, ok := Ration(goods, -250); eat != nil || fed != 250 || !ok {
		t.Fatalf("credit: %v %d %v", eat, fed, ok)
	}
	if eat, fed, ok := Ration(nil, 10); eat != nil || fed != 0 || ok {
		t.Fatalf("no goods: %v %d %v", eat, fed, ok)
	}
	// Overshoot stays below the largest unit eaten.
	for need := 1; need < 3*item.Meal.Nutrition(); need += 137 {
		eat, fed, ok := Ration([]item.Stack{{Kind: item.Meal, Qty: 5}, {Kind: item.Berries, Qty: 20}}, need)
		if !ok || fed < 0 {
			t.Fatalf("need %d: fed %d ok %v", need, fed, ok)
		}
		maxUnit := 0
		for _, s := range eat {
			if u := s.Kind.Nutrition(); u > maxUnit {
				maxUnit = u
			}
		}
		if fed >= maxUnit {
			t.Fatalf("need %d: fed %d not below unit %d", need, fed, maxUnit)
		}
		if Nutrition(eat)-need != fed {
			t.Fatalf("need %d: eaten %d fed %d", need, Nutrition(eat), fed)
		}
	}
}

func TestEatsBefore(t *testing.T) {
	if eatsBefore(item.SurvivalMeal, item.Meal) || !eatsBefore(item.Meal, item.SurvivalMeal) {
		t.Fatal("never-spoiling food goes last")
	}
	if !eatsBefore(item.Meal, item.Pemmican) || eatsBefore(item.Pemmican, item.Meal) {
		t.Fatal("shorter shelf first")
	}
}

func TestRationKindTieBreak(t *testing.T) {
	if item.Food.ShelfTicks() != 0 || item.SurvivalMeal.ShelfTicks() != 0 {
		t.Skip("premise: both kinds never spoil")
	}
	goods := []item.Stack{{Kind: item.SurvivalMeal, Qty: 2}, {Kind: item.Food, Qty: 2}}
	eat, _, ok := Ration(goods, 3*item.Food.Nutrition())
	want := []item.Stack{{Kind: item.Food, Qty: 2}, {Kind: item.SurvivalMeal, Qty: 1}}
	if !ok || !reflect.DeepEqual(eat, want) {
		t.Fatalf("lower kind first among never-spoil: %v", eat)
	}
	if !eatsBefore(item.Food, item.SurvivalMeal) || eatsBefore(item.SurvivalMeal, item.Food) {
		t.Fatal("tie-break by kind ascending")
	}
}

func TestRationOrderIndependent(t *testing.T) {
	goods := []item.Stack{
		{Kind: item.SurvivalMeal, Qty: 2}, {Kind: item.Meal, Qty: 1}, {Kind: item.Berries, Qty: 7},
		{Kind: item.Pemmican, Qty: 30}, {Kind: item.Meal, Qty: 2}, {Kind: item.Hay, Qty: 5}, {Kind: item.Berries, Qty: 3},
	}
	orig := append([]item.Stack(nil), goods...)
	need := 4*item.Meal.Nutrition() + 1234
	wantEat, wantFed, wantOK := Ration(goods, need)
	if !reflect.DeepEqual(goods, orig) {
		t.Fatal("goods was mutated")
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		sh := append([]item.Stack(nil), goods...)
		r.Shuffle(len(sh), func(a, b int) { sh[a], sh[b] = sh[b], sh[a] })
		eat, fed, ok := Ration(sh, need)
		if fed != wantFed || ok != wantOK || !reflect.DeepEqual(eat, wantEat) {
			t.Fatalf("shuffle %d differs: %v %d %v vs %v %d %v", i, eat, fed, ok, wantEat, wantFed, wantOK)
		}
	}
}
