package trade

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

func TestExoticTraderStocksCratedFurniture(t *testing.T) {
	seen := map[construction.Kind]bool{}
	for seed := uint64(0); seed < 60; seed++ {
		for _, l := range Generate(CurioMerchant, faction.Spacer, randx.New(seed)).Lines {
			if l.Building != 0 {
				if l.Kind != item.MinifiedBuilding || CrateValue(construction.Kind(l.Building)) <= 0 {
					t.Fatalf("crate line %+v", l)
				}
				seen[construction.Kind(l.Building)] = true
			}
		}
	}
	for _, k := range []construction.Kind{construction.BigScreenTV, construction.Telescope, construction.Urn} {
		if !seen[k] {
			t.Fatalf("the exotic trader never stocks a crated %v", k)
		}
	}
	for seed := uint64(0); seed < 60; seed++ {
		for _, l := range Generate(Quartermaster, faction.Industrial, randx.New(seed)).Lines {
			if l.Building != 0 {
				t.Fatalf("the quartermaster sells crates: %+v", l)
			}
		}
	}
}

func TestCratePriceSettleAndCodec(t *testing.T) {
	tv := Goods{item.MinifiedBuilding, uint8(construction.BigScreenTV)}
	if tv.Value() != 1485 || CrateValue(construction.Telescope) != 350 || CrateValue(construction.Bed) != 0 {
		t.Fatal("wiki crate values")
	}
	if PriceOf(tv, CurioMerchant, Buy, Terms{}) != 1485*100*(100+BuyMarkupPct)/100 {
		t.Fatalf("buy price %d", PriceOf(tv, CurioMerchant, Buy, Terms{}))
	}
	if Wants(CurioMerchant, item.MinifiedBuilding) {
		t.Fatal("the colony's crates are not bought back yet")
	}
	st := Stock{Kind: CurioMerchant}
	st.AddGoods(tv, 1)
	st.Add(item.Gold, 5)
	st.AddGoods(Goods{item.MinifiedBuilding, uint8(construction.Telescope)}, 1)
	if len(st.Lines) != 3 || st.Lines[1].Building != uint8(construction.BigScreenTV) || st.Lines[2].Building != uint8(construction.Telescope) {
		t.Fatalf("lines sort by kind then building: %+v", st.Lines)
	}
	col := map[item.Kind]int{item.Silver: 5000}
	r := Settle([]Delta{{Kind: item.MinifiedBuilding, Qty: 1, Building: tv.Building}}, col, st, Terms{}, false)
	if !r.OK() || len(r.Buy) != 1 || r.Buy[0].Building != tv.Building || r.Silver >= 0 {
		t.Fatalf("buying the crate: %+v", r)
	}
	if r := Settle([]Delta{{Kind: item.MinifiedBuilding, Qty: 2, Building: tv.Building}}, col, st, Terms{}, false); r.Reject != RejectOverStock {
		t.Fatalf("one in stock: %+v", r)
	}
	offer := []Delta{{Kind: item.Gold, Qty: 1}, {Kind: item.MinifiedBuilding, Qty: 1, Building: tv.Building}}
	enc := EncodeOffer(offer)
	dec, err := DecodeOffer(enc)
	if err != nil || len(dec) != 2 || dec[1] != offer[1] {
		t.Fatalf("codec %q: %+v %v", enc, dec, err)
	}
	if _, err := DecodeOffer("3/5:1"); err == nil {
		t.Fatal("only a crate carries a building")
	}
	g, ok := ParseGoods(tv.Name())
	if !ok || g != tv || tv.Label() != "Big screen television (crated)" {
		t.Fatalf("name %q label %q parse %+v", tv.Name(), tv.Label(), g)
	}
	if CrateStuff(0) != 0 || CrateStuff(tv.Building) == 0 {
		t.Fatal("only a crate has a crate material")
	}
	if g, ok := ParseGoods("steel"); !ok || g != (Goods{Kind: item.Steel}) {
		t.Fatal("plain goods parse")
	}
}
