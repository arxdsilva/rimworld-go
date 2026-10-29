package craft

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// weatherDays runs n days of hourly condition passes from tick 0.
func weatherDays(c *Craft, n int) {
	for h := range n * clock.HoursPerDay {
		c.Condition.Weather(uint64(h) * clock.TicksPerHour)
	}
}

func TestDeteriorationOutdoorsRoofedAndRain(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.put(1, item.Stack{Kind: item.Club, Qty: 1, X: 1, Y: 1})                    // outdoors, wood
	f.put(2, item.Stack{Kind: item.Club, Qty: 1, X: 2, Y: 2})                    // under a roof
	f.put(3, item.Stack{Kind: item.Club, Qty: 1, X: 3, Y: 3, Stuff: item.Steel}) // immune
	f.put(4, item.Stack{Kind: item.Wood, Qty: 50, X: 4, Y: 4})                   // raw: keeps
	f.put(5, item.Stack{Kind: item.Club, Qty: 1, X: 5, Y: 5, Carrier: pawn})
	f.roofed = map[[2]int]bool{{2, 2}: true}
	weatherDays(c, 3)
	if w := f.stacks[1].Wear; w != 3*item.DeteriorationPerDay/1000 {
		t.Fatalf("outdoor club lost %d HP in 3 days", w)
	}
	for _, id := range []entity.EntityID{2, 3, 4, 5} {
		if f.stacks[id].Wear != 0 {
			t.Errorf("stack %d lost %d HP", id, f.stacks[id].Wear)
		}
	}
	c.Condition.Weather(1) // between hours: nothing
	f.rain = true
	weatherDays(c, 1)
	if w := f.stacks[1].Wear; w != 3*2+item.DeteriorationPerDay*item.RainFactor/1000 {
		t.Fatalf("a rainy day: %d HP lost in all", w)
	}
}

func TestDeterioratedThingFallsApart(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.put(1, item.Stack{Kind: item.Club, Qty: 1, X: 1, Y: 1, Wear: 64})
	f.put(2, item.Stack{Kind: item.Club, Qty: 1, X: 2, Y: 1, Wear: 64, Quality: quality.Stored(quality.Awful)})
	f.claims[2] = pawn // reserved: kept until free
	f.rain = true
	weatherDays(c, 1)
	if _, ok := f.stacks[1]; ok {
		t.Fatal("a club at 0 HP is still there")
	}
	if _, ok := f.stacks[2]; !ok {
		t.Fatal("a reserved club was removed")
	}
	if len(f.msgs) != 1 || !strings.Contains(f.msgs[0], "club left outside fell apart") {
		t.Fatalf("messages %v", f.msgs)
	}
}

func TestWornApparelWearsOut(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	const a, dead entity.EntityID = 24, 48 // both have their hour at 0
	f.pawns = []entity.EntityID{a, dead}
	f.names[a] = "Ada"
	f.pos = map[entity.EntityID][2]int{a: {3, 3}}
	o := Outfit{}
	o.On[item.Body] = Piece{Kind: item.Tuque, Stuff: item.Cloth}
	o.SetForced(item.Body, true)
	f.SetOutfit(a, o)
	f.SetOutfit(dead, o)
	weatherDays(c, 40)
	w := f.outfits[a].On[item.Body].Wear
	if w < 8 || w > 24 { // 40 % of 40 days
		t.Fatalf("worn 40 days: %d HP lost", w)
	}
	if f.outfits[dead].On[item.Body].Wear != 0 {
		t.Fatal("the dead wear nothing out")
	}
	weatherDays(c, 400)
	if got := f.outfits[a]; got.On[item.Body].Kind != 0 || got.IsForced(item.Body) {
		t.Fatalf("a tuque worn 440 days: %+v", got)
	}
	if !strings.Contains(strings.Join(f.msgs, "|"), "Ada's tuque wore out.") {
		t.Fatalf("messages %v", f.msgs)
	}
}

// TestRotMovedFromGame: spoiled food goes once free, a human corpse
// becomes bones, and each kind lost is told once.
func TestRotMovedFromGame(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	shelf := item.Meat.ShelfTicks()
	f.put(1, item.Stack{Kind: item.Meat, Qty: 5, X: 1, Y: 1, Rot: shelf})
	f.put(2, item.Stack{Kind: item.Meat, Qty: 3, X: 2, Y: 1, Rot: shelf})
	f.put(3, item.Stack{Kind: item.Meat, Qty: 2, Carrier: pawn, Rot: shelf})
	f.put(4, item.Stack{Kind: item.CorpseHuman, Qty: 1, Of: 9, Rot: 2 * item.CorpseHuman.ShelfTicks()})
	f.put(5, item.Stack{Kind: item.Wood, Qty: 9})
	f.pos = map[entity.EntityID][2]int{pawn: {7, 7}}
	c.Condition.Rot(250)
	if _, ok := f.stacks[1]; ok {
		t.Fatal("rotten meat kept")
	}
	if _, ok := f.stacks[3]; !ok {
		t.Fatal("carried meat removed")
	}
	if b := f.stacks[4]; b.Kind != item.Bones || b.Of != 9 {
		t.Fatalf("corpse %+v", b)
	}
	if len(f.msgs) != 1 || f.msgs[0] != "1,1 8 meat rotted away" {
		t.Fatalf("messages %v", f.msgs)
	}
	f.temp = 0 // frozen: no rot
	before := f.stacks[3].Rot
	c.Condition.Rot(250)
	if f.stacks[3].Rot != before {
		t.Fatal("frozen food rots")
	}
	if FreshDays(item.Stack{Kind: item.Wood}) != -1 || FreshDays(item.Stack{Kind: item.Meat}) <= 0 {
		t.Fatal("FreshDays")
	}
}
