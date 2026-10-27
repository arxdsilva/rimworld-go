package trade

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

func allKinds() []item.Kind {
	var ks []item.Kind
	for k := item.Kind(0); k.String() != "unknown"; k++ {
		ks = append(ks, k)
	}
	return ks
}

func TestPriceBuyAboveSell(t *testing.T) {
	for _, k := range []TraderKind{BulkHauler, Quartermaster, CurioMerchant} {
		if b, s := Price(item.Rifle, k, Buy, Terms{}), Price(item.Rifle, k, Sell, Terms{}); b != 17400 || s >= b || s <= 0 {
			t.Fatalf("kind %d rifle buy %d sell %d", k, b, s)
		}
	}
}

func TestNoArbitrageAllKinds(t *testing.T) {
	for _, it := range allKinds() {
		if it.Value() <= 0 || it == item.Silver {
			continue
		}
		for k := TraderKind(0); k < NumKinds; k++ {
			for soc := 0; soc <= 20; soc++ {
				for gw := -100; gw <= 100; gw += 5 {
					tm := Terms{Social: soc, Goodwill: gw}
					b, s := Price(it, k, Buy, tm), Price(it, k, Sell, tm)
					if s >= b || s*100 > b*SellCapPct {
						t.Fatalf("%v kind %d social %d gw %d: buy %d sell %d", it, k, soc, gw, b, s)
					}
				}
			}
		}
	}
}

func TestPriceSocialMonotonic(t *testing.T) {
	pb, ps := 1<<30, 0
	for soc := 0; soc <= 20; soc++ {
		b := Price(item.Rifle, Quartermaster, Buy, Terms{Social: soc})
		s := Price(item.Rifle, Quartermaster, Sell, Terms{Social: soc})
		if b > pb || s < ps {
			t.Fatalf("social %d: buy %d (prev %d) sell %d (prev %d)", soc, b, pb, s, ps)
		}
		pb, ps = b, s
	}
}

func TestPriceGoodwillMonotonic(t *testing.T) {
	pb, ps := 1<<30, 0
	for gw := -100; gw <= 100; gw++ {
		b := Price(item.Parka, BulkHauler, Buy, Terms{Goodwill: gw})
		s := Price(item.Parka, BulkHauler, Sell, Terms{Goodwill: gw})
		if b > pb || s < ps {
			t.Fatalf("goodwill %d: buy %d sell %d", gw, b, s)
		}
		pb, ps = b, s
	}
	if Price(item.Parka, BulkHauler, Buy, Terms{Goodwill: 1000, Social: 20}) != 80*106 {
		t.Fatal("buy floor is 6%")
	}
}

func TestSilverAtPar(t *testing.T) {
	for _, tm := range []Terms{{}, {Social: 20, Goodwill: 100}, {Goodwill: -100}, {Hostile: true}} {
		for _, s := range []Side{Buy, Sell} {
			if p := Price(item.Silver, CurioMerchant, s, tm); p != 100 {
				t.Fatalf("silver %v = %d", tm, p)
			}
		}
	}
	if Price(item.Parka, BulkHauler, Buy, Terms{Hostile: true}) != 0 {
		t.Fatal("hostile has no price")
	}
}

func TestSpecialtyPaysMore(t *testing.T) {
	// Steel is bulk-hauler specialty (Materials), not the curio's.
	if Price(item.Steel, BulkHauler, Sell, Terms{}) <= Price(item.Steel, CurioMerchant, Sell, Terms{}) {
		t.Fatal("specialty should pay more")
	}
}

func TestGenerateStockDeterministic(t *testing.T) {
	for k := TraderKind(0); k < NumKinds; k++ {
		a := Generate(k, faction.Industrial, randx.New(7))
		b := Generate(k, faction.Industrial, randx.New(7))
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("kind %d differs", k)
		}
	}
}

func has(st Stock, it item.Kind) bool {
	for _, l := range st.Lines {
		if l.Kind == it {
			return true
		}
	}
	return false
}

func TestGenerateStockPerKind(t *testing.T) {
	for k := TraderKind(0); k < NumKinds; k++ {
		d := DefOf(k)
		for seed := uint64(0); seed < 40; seed++ {
			for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial} {
				st := Generate(k, tech, randx.New(seed))
				if st.Silver < d.SilverMin || st.Silver > d.SilverMax || len(st.Lines) == 0 {
					t.Fatalf("kind %d silver %d lines %d", k, st.Silver, len(st.Lines))
				}
				for i, l := range st.Lines {
					if l.Qty <= 0 || l.Kind == item.Silver || i > 0 && st.Lines[i-1].Kind >= l.Kind {
						t.Fatalf("bad line %+v", l)
					}
				}
				if tech == faction.Neolithic && (has(st, item.Rifle) || has(st, item.Components)) {
					t.Fatal("neolithic carries rifles or components")
				}
			}
		}
	}
	if !has(Generate(Quartermaster, faction.Industrial, randx.New(1)), item.Rifle) {
		t.Fatal("industrial quartermaster sells rifles")
	}
	if !has(Generate(CurioMerchant, faction.Spacer, randx.New(1)), item.Gold) {
		t.Fatal("curio sells gold")
	}
	if DefOf(NumKinds).Key != "" || Generate(NumKinds, faction.Spacer, randx.New(1)).Silver != 0 {
		t.Fatal("unknown kind")
	}
	if !DefOf(CurioMerchant).Orbital || DefOf(BulkHauler).Orbital {
		t.Fatal("orbital flags")
	}
}

func TestWants(t *testing.T) {
	if !Wants(BulkHauler, item.Wood) || Wants(BulkHauler, item.Rifle) || Wants(BulkHauler, item.Silver) ||
		!Wants(Quartermaster, item.Rifle) || Wants(Quartermaster, item.Beer) || !Wants(CurioMerchant, item.Gold) ||
		Wants(CurioMerchant, item.Wood) || Wants(NumKinds, item.Wood) || Wants(BulkHauler, item.CorpseHuman) {
		t.Fatal("wants table")
	}
}

func stock() Stock {
	return Stock{Kind: Quartermaster, Silver: 500, Lines: []Line{{item.Rifle, 2}, {item.Parka, 3}}}
}

func TestSettleBuyAndSell(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 400, item.Club: 5}
	r := Settle([]Delta{{item.Parka, 1}, {item.Club, -2}}, col, stock(), Terms{}, false)
	if !r.OK() {
		t.Fatalf("rejected: %v", r.Reject)
	}
	want := 2*Price(item.Club, Quartermaster, Sell, Terms{}) - Price(item.Parka, Quartermaster, Buy, Terms{})
	if r.SilverCenti != want || len(r.Buy) != 1 || len(r.Sell) != 1 {
		t.Fatalf("%+v want %d", r, want)
	}
	// Colony pays: rounded up, away from the colony.
	if r.Silver > 0 || r.Silver != -((-want+99)/100) {
		t.Fatalf("silver %d", r.Silver)
	}
	// Colony receives.
	r = Settle([]Delta{{item.Club, -5}}, col, stock(), Terms{}, false)
	if !r.OK() || r.Silver != r.SilverCenti/100 || r.Silver <= 0 {
		t.Fatalf("%+v", r)
	}
}

func TestSettleRejectsUnaffordable(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 10, item.Club: 100}
	if r := Settle([]Delta{{item.Rifle, 1}}, col, stock(), Terms{}, false); r.Reject != RejectUnaffordable {
		t.Fatalf("colony: %v", r.Reject)
	}
	if r := Settle([]Delta{{item.Club, -100}}, col, stock(), Terms{}, false); r.Reject != RejectUnaffordable {
		t.Fatalf("trader: %v", r.Reject)
	}
}

func TestSettleRejectsOverStock(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 99999, item.Club: 1}
	if r := Settle([]Delta{{item.Rifle, 3}}, col, stock(), Terms{}, false); r.Reject != RejectOverStock {
		t.Fatalf("trader stock: %v", r.Reject)
	}
	if r := Settle([]Delta{{item.Club, -2}}, col, stock(), Terms{}, false); r.Reject != RejectOverStock {
		t.Fatalf("colony stock: %v", r.Reject)
	}
}

func TestSettleRejectsUnwanted(t *testing.T) {
	col := map[item.Kind]int{item.Wood: 50, item.Silver: 50}
	if r := Settle([]Delta{{item.Wood, -5}}, col, stock(), Terms{}, false); r.Reject != RejectUnwanted {
		t.Fatalf("%v", r.Reject)
	}
	if r := Settle([]Delta{{item.Silver, 5}}, col, stock(), Terms{}, false); r.Reject != RejectUnwanted {
		t.Fatalf("silver line: %v", r.Reject)
	}
}

func TestSettleHostileEmpty(t *testing.T) {
	col := map[item.Kind]int{item.Club: 5}
	if r := Settle([]Delta{{item.Club, -1}}, col, stock(), Terms{Hostile: true}, false); r.Reject != RejectHostile {
		t.Fatal("hostile")
	}
	if r := Settle([]Delta{{item.Club, 1}, {item.Club, -1}}, col, stock(), Terms{}, false); r.Reject != RejectEmpty {
		t.Fatal("empty")
	}
	for _, r := range []Reject{RejectNone, RejectEmpty, RejectHostile, RejectUnaffordable, RejectOverStock, RejectUnwanted} {
		if (r == RejectNone) != (r.String() == "") {
			t.Fatalf("string %d", r)
		}
	}
}

func TestSettleGiftNoSilver(t *testing.T) {
	col := map[item.Kind]int{item.Silver: 300, item.Wood: 500}
	// Gifts need not be wanted and pay nothing back.
	r := Settle([]Delta{{item.Silver, -200}, {item.Wood, -100}}, col, stock(), Terms{}, true)
	if !r.OK() || r.SilverCenti != 0 || r.Silver != 0 || r.Goodwill != faction.GiftGoodwill(300) || len(r.Sell) != 2 {
		t.Fatalf("%+v", r)
	}
	if GiftValue([]Delta{{item.Silver, -200}, {item.Wood, -100}, {item.Rifle, 4}}) != 300 {
		t.Fatal("gift value counts only what the colony gives")
	}
	if r := Settle([]Delta{{item.Rifle, 1}}, col, stock(), Terms{}, true); r.Reject != RejectUnwanted {
		t.Fatal("cannot take from a gift")
	}
	if r := Settle([]Delta{{item.Silver, -400}}, col, stock(), Terms{}, true); r.Reject != RejectOverStock {
		t.Fatal("gift over colony silver")
	}
	if r := Settle([]Delta{{item.Bones, -1}}, map[item.Kind]int{item.Bones: 1}, stock(), Terms{}, true); r.Reject != RejectUnwanted {
		t.Fatal("worthless gift")
	}
}
