// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"
	"sync"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

var (
	foodPlanetOnce sync.Once
	foodPlanet     *planet.Planet
)

// testWorld is a small planet, built once (read-only: Route has its own
// scratch per fork).
func testWorld() *planet.Planet {
	foodPlanetOnce.Do(func() {
		foodPlanet = planet.Build(planet.Params{SeedText: "caravan", Coverage: 30, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()})
	})
	return foodPlanet.Fork()
}

// foodHost is a Host for the food and boarding tests.
type foodHost struct {
	Host
	captives, standing map[entity.EntityID]bool
	noStash            map[entity.EntityID]bool
	breaking           map[entity.EntityID]bool
	capacity           map[entity.EntityID]int32
	stashed            []entity.EntityID
	goods              []item.Stack
	planet             *planet.Planet
	tile               int
	letters            int
}

func (h *foodHost) Captive(id entity.EntityID) bool  { return h.captives[id] }
func (h *foodHost) Standing(id entity.EntityID) bool { return h.standing[id] }
func (h *foodHost) Breaking(id entity.EntityID) bool { return h.breaking[id] }
func (h *foodHost) Candidate(id entity.EntityID) bool {
	return h.standing[id] && !h.captives[id]
}
func (h *foodHost) Capacity(id entity.EntityID, _ body.Capacity) int32 { return h.capacity[id] }
func (h *foodHost) Stash(id, _ entity.EntityID) bool {
	if h.noStash[id] {
		return false
	}
	h.stashed = append(h.stashed, id)
	return true
}
func (h *foodHost) Name(entity.EntityID) string { return "x" }
func (h *foodHost) Planet() *planet.Planet      { return h.planet }
func (h *foodHost) ColonyTile() int             { return h.tile }
func (h *foodHost) Current() uint32             { return 0 }
func (h *foodHost) Cargo() Freight              { return &foodCargo{h: h} }

// foodCargo is the host's goods, eaten by Yield.
type foodCargo struct {
	Freight
	h *foodHost
}

func (c *foodCargo) Pace(uint64, int) int      { return 1000 }
func (c *foodCargo) Goods(uint64) []item.Stack { return slices.Clone(c.h.goods) }
func (c *foodCargo) Yield(_ uint64, k item.Kind, qty int) bool {
	for i := range c.h.goods {
		if c.h.goods[i].Kind == k && c.h.goods[i].Qty >= qty {
			c.h.goods[i].Qty -= qty
			return true
		}
	}
	return false
}

func newFoodHost() *foodHost {
	return &foodHost{captives: map[entity.EntityID]bool{}, standing: map[entity.EntityID]bool{}, noStash: map[entity.EntityID]bool{},
		breaking: map[entity.EntityID]bool{}, capacity: map[entity.EntityID]int32{}}
}

func fleetFor(h Host, list *[]Caravan) Fleet { return Of(list, &Atlas{}, h) }

// TestBoardTakesStandingCaptivesOnly: a captive boards when it is still a
// prisoner, standing and can be stashed; the rest stay home; Captives ends
// empty; duplicates and members board once.
func TestBoardTakesStandingCaptivesOnly(t *testing.T) {
	h := newFoodHost()
	for _, id := range []entity.EntityID{5, 6, 7, 8, 9} {
		h.captives[id], h.standing[id] = true, true
	}
	h.standing[6] = false // downed since
	h.captives[7] = false // freed since
	h.noStash[8] = true   // cannot leave the map
	list := []Caravan{{C: caravan.Caravan{ID: 1, Members: []uint64{2, 9}}, Captives: []entity.EntityID{9, 8, 7, 6, 5, 5}}}
	c := &list[0]
	board(fleetFor(h, &list), c)
	if want := []uint64{2, 9, 5}; !slices.Equal(c.C.Members, want) {
		t.Fatalf("members = %v, want %v", c.C.Members, want)
	}
	if !slices.Equal(h.stashed, []entity.EntityID{5}) {
		t.Fatalf("stashed = %v, want only 5 (9 was already aboard)", h.stashed)
	}
	if c.Captives != nil {
		t.Fatalf("captives = %v, want nil", c.Captives)
	}
}

// TestBoardLooksAtMostMaxCaptives: a hostile save cannot make board stash a
// long list.
func TestBoardLooksAtMostMaxCaptives(t *testing.T) {
	h := newFoodHost()
	var ids []entity.EntityID
	for i := entity.EntityID(100); i < 100+2*maxCaptives; i++ {
		h.captives[i], h.standing[i] = true, true
		ids = append(ids, i)
	}
	list := []Caravan{{C: caravan.Caravan{ID: 1}, Captives: ids}}
	board(fleetFor(h, &list), &list[0])
	if len(h.stashed) != maxCaptives {
		t.Fatalf("stashed %d, want %d", len(h.stashed), maxCaptives)
	}
}

// TestPickCaptivesLeavesOutTheTakenAndTheDowned: prisoners already in a
// caravan (member or captive), downed, breaking or colonists are left out.
func TestPickCaptivesLeavesOutTheTakenAndTheDowned(t *testing.T) {
	h := newFoodHost()
	for _, id := range []entity.EntityID{1, 2, 3, 4, 5, 6} {
		h.captives[id], h.standing[id] = true, true
	}
	h.standing[2] = false
	h.breaking[3] = true
	list := []Caravan{{C: caravan.Caravan{Members: []uint64{4}}, Captives: []entity.EntityID{5}}}
	f := fleetFor(h, &list)
	got := pickCaptives(f, "6,5,4,3,2,1,junk,bread:2", []entity.EntityID{6})
	if want := []entity.EntityID{1}; !slices.Equal(got, want) {
		t.Fatalf("captives = %v, want %v", got, want)
	}
	if !listed(f, 4) || !listed(f, 5) || listed(f, 1) {
		t.Fatal("listed: members and captives count, others do not")
	}
}

// TestEatTakesWholeUnitsThenStarves: each hour the members eat whole units
// off the cargo, the surplus is kept in Fed; with too little left what is
// left is eaten and eat says false.
func TestEatTakesWholeUnitsThenStarves(t *testing.T) {
	h := newFoodHost()
	h.goods = []item.Stack{{Kind: item.Meal, Qty: 3}}
	list := []Caravan{{C: caravan.Caravan{ID: 1, Members: []uint64{1, 2}}}}
	f, c := fleetFor(h, &list), &list[0]
	need := caravan.HourNeed(2)
	unit := item.Meal.Nutrition()
	units := 0
	for i := 0; i < 3; i++ {
		before := h.goods[0].Qty
		if !eat(f, c) {
			break
		}
		units += before - h.goods[0].Qty
	}
	if want := (3*need + unit - 1) / unit; units > want || units < 1 {
		t.Fatalf("ate %d units in 3 hours, want at most %d", units, want)
	}
	if int(c.Fed) >= unit || c.Fed < 0 {
		t.Fatalf("Fed = %d, want in [0, %d)", c.Fed, unit)
	}
	for eat(f, c) {
	}
	if h.goods[0].Qty != 0 || c.Fed != 0 {
		t.Fatalf("starving caravan: cargo %v fed %d, want all eaten and 0", h.goods, c.Fed)
	}
	if eat(f, c) {
		t.Fatal("an empty cargo cannot feed anyone")
	}
}

// TestFoodDaysCountsReservePlusCargo: the shown days are the reserve plus
// what the cargo (and Fed) feeds the eaters.
func TestFoodDaysCountsReservePlusCargo(t *testing.T) {
	h := newFoodHost()
	h.goods = []item.Stack{{Kind: item.Meal, Qty: 4}, {Kind: item.Hay, Qty: 99}}
	list := []Caravan{{C: caravan.Caravan{ID: 1, Members: []uint64{1, 2}, FoodDays: 500}, Fed: 10}}
	f := fleetFor(h, &list)
	want := 500 + int(caravan.Days(4*item.Meal.Nutrition()+10, 2))
	if got := foodDays(f, &list[0]); got != want {
		t.Fatalf("foodDays = %d, want %d", got, want)
	}
	if v := Views(f, func(entity.EntityID) string { return "" }); v[0].FoodDays != want/100 {
		t.Fatalf("view FoodDays = %d, want %d", v[0].FoodDays, want/100)
	}
}

// TestSlowestIsTheSlowestWalker: the downed (0) are carried and do not
// count; with nobody walking it is the full 1000; never below 1.
func TestSlowestIsTheSlowestWalker(t *testing.T) {
	h := newFoodHost()
	h.capacity[1], h.capacity[2], h.capacity[3] = 800, 0, 400
	if got := slowest(h, []uint64{1, 2, 3}); got != 400 {
		t.Fatalf("slowest = %d, want 400", got)
	}
	if got := slowest(h, []entity.EntityID{2}); got != 1000 {
		t.Fatalf("slowest of the downed = %d, want 1000", got)
	}
	if got := slowest(h, []uint64(nil)); got != 1000 {
		t.Fatalf("slowest of nobody = %d, want 1000", got)
	}
}

// TestPlanPinnedReasons: every reason Plan gives, and a real estimate that
// slows with load and with a slow walker.
func TestPlanPinnedReasons(t *testing.T) {
	h := newFoodHost()
	h.standing[1], h.capacity[1] = true, 1000
	var list []Caravan
	f := fleetFor(h, &list)
	members := []entity.EntityID{1}
	if _, why := Plan(f, members, 3, 0, 0); why != "No world map." {
		t.Fatalf("no planet: %q", why)
	}
	p := testWorld()
	h.planet = p
	home := p.RandomLanding(0)
	h.tile = home
	for _, dest := range []int{-1, p.TileCount(), home} {
		if _, why := Plan(f, members, dest, 0, 0); why != "Pick a destination for the caravan." {
			t.Fatalf("dest %d: %q", dest, why)
		}
	}
	far := -1
	for _, nb := range p.Neighbours(home) {
		for _, nn := range p.Neighbours(int(nb)) {
			if r := p.Route(home, int(nn)); len(r) == 3 {
				far = int(nn)
			}
		}
	}
	if far < 0 {
		t.Skip("no tile two steps from home")
	}
	fast, why := Plan(f, members, far, 0, 0)
	if why != "" || fast <= 0 {
		t.Fatalf("estimate = %d %q", fast, why)
	}
	if loaded, _ := Plan(f, members, far, 5000, 0); loaded < fast { // load clamps to 1000
		t.Fatalf("a full load (%d) is not faster than none (%d)", loaded, fast)
	}
	h.capacity[1] = 200
	if slow, why := Plan(f, members, far, -9, 0); why == "" && slow < fast {
		t.Fatalf("a slow walker (%d) is faster than a fast one (%d)", slow, fast)
	}
	h.capacity[1] = 1
	if _, why := Plan(f, members, far, 1000, 0); why != "" && why != "Too slow to get there." {
		t.Fatalf("hopeless caravan: %q", why)
	}
}

func (h *foodHost) Close(uint32)          {}
func (h *foodHost) Now() uint64           { return 0 }
func (h *foodHost) TileName(t int) string { return "tile" }
func (h *foodHost) Letter(bool, string, entity.EntityID, string, ...any) {
	h.letters++
}

// TestCloseKeepsReserveAndCargoFood: a caravan still forming on a closing
// map sets out with the reserve its record holds and the cargo it packed,
// and its waiting prisoners stay behind.
func TestCloseKeepsReserveAndCargoFood(t *testing.T) {
	h := newFoodHost()
	h.goods = []item.Stack{{Kind: item.Meal, Qty: 3}}
	h.captives[9], h.standing[9] = true, true
	list := []Caravan{{C: caravan.Caravan{ID: 1, Members: []uint64{2}, FoodDays: 777, State: caravan.Forming}, Map: 4, Captives: []entity.EntityID{9}}}
	f := Of(&list, &Atlas{}, h)
	f.Life().close(4)
	c := list[0]
	if c.C.State != caravan.Travelling || c.C.FoodDays != 777 || h.goods[0].Qty != 3 {
		t.Fatalf("caravan %+v goods %v: the reserve and the cargo must survive the close", c.C, h.goods)
	}
	if len(h.stashed) != 0 || c.Captives != nil || len(c.C.Members) != 1 {
		t.Fatalf("a closing map's prisoner boarded: stashed %v members %v", h.stashed, c.C.Members)
	}
	if h.letters != 1 {
		t.Fatalf("%d departure letters", h.letters)
	}
}

// TestPlanNoWayThere: a destination no route reaches gives the pinned
// reason (the test planet has sea ice and islands).
func TestPlanNoWayThere(t *testing.T) {
	h := newFoodHost()
	h.standing[1], h.capacity[1] = true, 1000
	p := testWorld()
	h.planet, h.tile = p, p.RandomLanding(0)
	unreachable := -1
	for i := 0; i < p.TileCount() && unreachable < 0; i++ {
		if i != h.tile && len(p.Route(h.tile, i)) < 2 {
			unreachable = i
		}
	}
	if unreachable < 0 {
		t.Skip("every tile is reachable on this planet")
	}
	var list []Caravan
	if _, why := Plan(fleetFor(h, &list), []entity.EntityID{1}, unreachable, 0, 0); why != "No way to get there." {
		t.Fatalf("unreachable tile %d: %q", unreachable, why)
	}
}

// TestHungerUnitsMatchTheDomain pins the arithmetic the claims rely on.
func TestHungerUnitsMatchTheDomain(t *testing.T) {
	if caravan.HourNeed(2) != 2*needs.HungerPerHour {
		t.Fatal("two eaters eat twice an hour's need")
	}
}
