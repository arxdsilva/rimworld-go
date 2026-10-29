package craft

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

var craftOnly = work.Mask(0).Set(work.Craft, true)

// clubShop is a workbench with a club bill and the given materials.
func clubShop(stacks ...item.Stack) (*fakeHost, *Craft) {
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	for i, st := range stacks {
		f.put(entity.EntityID(50+i), st)
	}
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.Club, Mode: recipe.DoCount, Target: 1}})
	return f, c
}

// made is the one made stack the fake saw dropped of kind k.
func made(t *testing.T, f *fakeHost, k item.Kind) item.Stack {
	t.Helper()
	for _, st := range f.drops {
		if st.Kind == k {
			return st
		}
	}
	t.Fatalf("no %v made (drops %v)", k, f.drops)
	return item.Stack{}
}

func TestCraftedClubRollsQualityAndKeepsItsStuff(t *testing.T) {
	t.Parallel()
	f, c := clubShop(item.Stack{Kind: item.Steel, Qty: 10, X: 1, Y: 1})
	if j := runBill(t, f, c, craftOnly, 100); j.State != job.Complete {
		t.Fatalf("job %v", j.State)
	}
	club := made(t, f, item.Club)
	if _, ok := quality.FromStored(club.Quality); !ok || club.Stuff != item.Steel || club.Qty != 1 {
		t.Fatalf("club %+v", club)
	}
	if f.rolls != 1 {
		t.Fatalf("%d quality rolls", f.rolls)
	}
}

func TestPlainOutputsRollNothing(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f)
	f.pawns = []entity.EntityID{pawn}
	f.build(1, construction.Workbench, 4, 4)
	f.put(50, item.Stack{Kind: item.Wood, Qty: 20})
	c.Bills.Set(1, []recipe.Bill{{Recipe: recipe.MakePegLeg, Mode: recipe.DoCount, Target: 1}})
	runBill(t, f, c, craftOnly, 2000)
	if leg := made(t, f, item.PegLeg); leg.Quality != 0 || leg.Stuff != 0 || f.rolls != 0 {
		t.Fatalf("peg leg %+v, %d rolls", leg, f.rolls)
	}
}

func TestInspiredCraftSpendsTheInspiration(t *testing.T) {
	t.Parallel()
	f, c := clubShop(item.Stack{Kind: item.Wood, Qty: 10})
	f.skills[pawn] = 20
	f.names[pawn] = "Ada"
	f.inspired = map[entity.EntityID]bool{pawn: true}
	runBill(t, f, c, craftOnly, 100)
	l, _ := quality.FromStored(made(t, f, item.Club).Quality)
	if l < quality.Excellent || f.inspired[pawn] {
		t.Fatalf("inspired skill-20 club %v, inspiration left %v", l, f.inspired)
	}
	if l >= AnnounceFrom && (len(f.msgs) == 0 || !strings.Contains(f.msgs[len(f.msgs)-1], "Ada made a")) {
		t.Fatalf("no announcement: %v", f.msgs)
	}
}

func TestQualityRollDistribution(t *testing.T) {
	t.Parallel()
	f := newFake()
	q := &Quality{h: f}
	f.skills[pawn] = 10
	var n [7]int
	const runs = 4000
	for range runs {
		n[q.Roll(pawn, 0)]++
	}
	// skill 10: normal 40 %, good 44 %, excellent 12 %; no legendary
	if n[quality.Legendary] != 0 || n[quality.Normal] < runs*30/100 || n[quality.Good] < runs*34/100 || n[quality.Excellent] < runs*7/100 {
		t.Fatalf("skill 10 spread %v", n)
	}
}

func TestBuiltQuality(t *testing.T) {
	t.Parallel()
	f := newFake()
	q := &Quality{h: f}
	if q.Built(pawn, construction.Wall, 0, 0) != 0 || q.Built(pawn, construction.Workbench, 0, 0) != 0 || f.rolls != 0 {
		t.Fatal("walls and benches have no quality")
	}
	for _, k := range []construction.Kind{construction.Bed, construction.Table, construction.Chair, construction.EndTable, construction.Bedroll} {
		if _, ok := quality.FromStored(q.Built(pawn, k, 0, 0)); !ok {
			t.Errorf("%v built with no quality", k)
		}
	}
	if QualityBuilding(construction.SleepingSpot) || QualityBuilding(construction.AnimalSleepingSpot) {
		t.Fatal("spots are not built")
	}
	f.inspired = map[entity.EntityID]bool{pawn: true}
	f.skills[pawn] = 20
	if l, _ := quality.FromStored(q.Built(pawn, construction.Bed, 3, 3)); l < quality.Excellent || len(f.inspired) != 0 {
		t.Fatalf("inspired bed %v", l)
	}
}

// TestStuffedBillTakesOneMaterial: a club takes the nearest material
// there is enough of, never a mix, and a pinned bill only its material.
func TestStuffedBillTakesOneMaterial(t *testing.T) {
	t.Parallel()
	f, c := clubShop(item.Stack{Kind: item.Wood, Qty: 2, X: 4, Y: 5}, item.Stack{Kind: item.Steel, Qty: 2, X: 4, Y: 6},
		item.Stack{Kind: item.GraniteBlocks, Qty: 3, X: 9, Y: 9}, item.Stack{Kind: item.GraniteBlocks, Qty: 3, X: 9, Y: 8})
	runBill(t, f, c, craftOnly, 200)
	if club := made(t, f, item.Club); club.Stuff != item.GraniteBlocks {
		t.Fatalf("club of %v", club.Stuff)
	}
	if f.Stock(item.Wood) != 2 || f.Stock(item.Steel) != 2 || f.Stock(item.GraniteBlocks) != 2 {
		t.Fatalf("left wood %d steel %d granite %d", f.Stock(item.Wood), f.Stock(item.Steel), f.Stock(item.GraniteBlocks))
	}

	f, c = clubShop(item.Stack{Kind: item.Wood, Qty: 10, X: 4, Y: 5}, item.Stack{Kind: item.Steel, Qty: 10, X: 9, Y: 9})
	c.Bills.Apply(command.Command{Kind: command.ConfigureBill, X: 4, Y: 4, Index: 0, Stuff: uint8(item.Steel)})
	if v := c.Bills.AppendViews(nil, 1); v[0].Stuff != "steel" || v[0].StuffKind != uint8(item.Steel) || !v[0].Quality {
		t.Fatalf("view %+v", v[0])
	}
	runBill(t, f, c, craftOnly, 200)
	if club := made(t, f, item.Club); club.Stuff != item.Steel {
		t.Fatalf("pinned club of %v", club.Stuff)
	}
}

func TestStuffedAvailability(t *testing.T) {
	t.Parallel()
	f, c := clubShop(item.Stack{Kind: item.Wood, Qty: 3}, item.Stack{Kind: item.Steel, Qty: 3})
	if c.Bills.AppendViews(nil, 1)[0].Status != BillNoInput {
		t.Fatal("3 wood and 3 steel make no club")
	}
	f.put(60, item.Stack{Kind: item.Steel, Qty: 1, X: 2})
	if st := c.Bills.AppendViews(nil, 1)[0].Status; st == BillNoInput {
		t.Fatal("4 steel in two piles make a club")
	}
}
