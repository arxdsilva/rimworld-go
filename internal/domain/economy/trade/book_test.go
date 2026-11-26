// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package trade

import (
	"fmt"
	"hash/fnv"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

// Plugin goods in trade (PLG-026): the run's item book prices, names and
// stocks a plugin kind; core trade reads the same numbers as before.

// coreStream is Generate's stock and the next draw after it, hashed over
// seeds 1..50 per trader kind and tech, recorded from the core-only body
// before plugin goods could be stocked (the W1-trade baseline).
// The key is "<trader kind>/<tech>"; historical kind 3 was the invalid sentinel.
var coreStream = map[string]string{
	"0/0": "535dbfb823013c4a", "0/1": "e46bcf3e73f41880", "0/2": "e46bcf3e73f41880",
	"1/0": "dd94a922cf15e3bb", "1/1": "101d652431835839", "1/2": "101d652431835839",
	"2/0": "7f168d4edde528cd", "2/1": "7f168d4edde528cd", "2/2": "7f168d4edde528cd",
	"3/0": "1e56e0d3def0d8ac", "3/1": "1e56e0d3def0d8ac", "3/2": "1e56e0d3def0d8ac",
}

// medicalAdded is the additive MED-14-only stock. The historical stream
// contract still covers every pre-MED-14 item, quantity and next draw.
func medicalAdded(k item.Kind) bool {
	switch k {
	case item.BlissCircuit, item.NerveDampener, item.AcidFailsafe,
		item.AcousticImplant, item.ServoClaw, item.CochlearImplant,
		item.ServoJaw, item.SynthTongue, item.ApexArm, item.ApexLeg,
		item.ApexEye:
		return true
	}
	return false
}

// streamHash hashes historical stock and the draw after it for seeds 1..50.
func streamHash(b item.Book, k TraderKind, tech faction.Tech) string {
	h := fnv.New64a()
	for seed := uint64(1); seed <= 50; seed++ {
		rng := randx.New(seed)
		st := Generate(b, k, tech, rng)
		// Preserve the pre-lot random-stream contract. New persisted
		// physical fields do not consume random draws.
		var lines []struct {
			Kind     item.Kind
			Qty      int
			Building uint8
		}
		for _, l := range st.Lines {
			if medicalAdded(l.Kind) {
				continue
			}
			lines = append(lines, struct {
				Kind     item.Kind
				Qty      int
				Building uint8
			}{l.Kind, l.Qty, l.Building})
		}
		fmt.Fprintf(h, "%d|%d|%v|%d;", seed, st.Silver, lines, rng.IntN(1<<30))
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// TestGenerateCoreStreamUnchanged pins the three original traveling merchants
// and the invalid kind's draw stream. New settlement kinds have their own
// deterministic generation tests; the historical invalid sentinel was 3.
func TestGenerateCoreStreamUnchanged(t *testing.T) {
	t.Parallel()
	books := map[string]item.Book{"core": {}, "no opt-in": testBook(item.Def{Label: "bell", Path: "Manufactured/Components", Value: 40})}
	for name, b := range books {
		for _, k := range []TraderKind{BulkHauler, Quartermaster, CurioMerchant, NumKinds} {
			for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial, faction.Spacer} {
				historical := k
				if k == NumKinds {
					historical = TribalSettlement // the former invalid kind was 3
				}
				key := fmt.Sprintf("%d/%d", historical, tech)
				if got := streamHash(b, k, tech); got != coreStream[key] {
					t.Errorf("%s book, %s (now %d): stream %q, want %q", name, key, k, got, coreStream[key])
				}
			}
		}
	}
}

// testBook is core's item kinds plus one plugin kind per def, keyed
// "test.p<i>".
func testBook(plugins ...item.Def) item.Book {
	core := item.Core()
	defs, keys := make([]item.Def, 0, core.Len()+len(plugins)), core.Keys()
	for k := range core.Len() {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	for i, d := range plugins {
		defs = append(defs, d)
		keys = append(keys, fmt.Sprintf("test.p%d", i))
	}
	return item.NewBook(defs, keys)
}

// TestWantsPriceByBook: a plugin kind trades at its def value and by its
// category. A Manufactured plugin good prices exactly as a core
// Manufactured good of the same value (straw carpet, 15) on every side,
// trader and terms; at value 18 its prices are the core formula's. The
// curio merchant wants it, the bulk hauler wants a Raw-resources plugin
// good (Materials), and a value-0 plugin good is wanted by nobody.
func TestWantsPriceByBook(t *testing.T) {
	t.Parallel()
	b := testBook(
		item.Def{Label: "carpet", Path: "Manufactured/Textiles", Value: item.CarpetStraw.Value()},
		item.Def{Label: "bell", Path: "Manufactured/Components", Value: 18},
		item.Def{Label: "ore", Path: "Raw resources/Metals", Value: 7},
		item.Def{Label: "dust", Path: "Manufactured/Components", Value: 0},
	)
	carpet, bell, ore, dust := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1), item.Kind(item.CoreKinds+2), item.Kind(item.CoreKinds+3)
	if item.CarpetStraw.Category() != "Manufactured" {
		t.Fatalf("straw carpet is %s", item.CarpetStraw.Category())
	}
	for k := TraderKind(0); k < NumKinds; k++ {
		if Wants(b, k, carpet) != Wants(item.Book{}, k, item.CarpetStraw) {
			t.Fatalf("trader %d: wants plugin carpet %v, core %v", k, Wants(b, k, carpet), Wants(item.Book{}, k, item.CarpetStraw))
		}
		for _, s := range []Side{Buy, Sell} {
			for _, tm := range []Terms{{}, {Social: 9, Goodwill: 40}, {Social: 20, Goodwill: 100}, {Goodwill: -80}, {Hostile: true}} {
				if p, c := Price(b, carpet, k, s, tm), Price(item.Book{}, item.CarpetStraw, k, s, tm); p != c {
					t.Fatalf("trader %d side %d terms %+v: plugin %d, core %d", k, s, tm, p, c)
				}
			}
		}
	}
	// A plugin good follows the same 140% buy / 60% sell Core coefficients.
	if buy, sell := Price(b, bell, CurioMerchant, Buy, Terms{}), Price(b, bell, CurioMerchant, Sell, Terms{}); buy != 2520 || sell != 1080 {
		t.Fatalf("bell at the curio: buy %d sell %d, want 2520 1080", buy, sell)
	}
	if sell := Price(b, bell, BulkHauler, Sell, Terms{}); sell != 1080 {
		t.Fatalf("bell at the bulk hauler sells %d, want 1080 (Core price unaffected by trader inventory category)", sell)
	}
	if !Wants(b, CurioMerchant, bell) || Wants(b, BulkHauler, bell) || Wants(b, Quartermaster, bell) {
		t.Fatal("only the curio merchant wants a Manufactured plugin good")
	}
	if !Wants(b, BulkHauler, ore) || Wants(b, CurioMerchant, ore) || Wants(b, Quartermaster, ore) {
		t.Fatal("only the bulk hauler wants a Raw-resources plugin good")
	}
	// A trader's specialty controls stocking choices; the common Core price applies.
	if Price(b, ore, BulkHauler, Sell, Terms{}) != Price(b, ore, Quartermaster, Sell, Terms{}) {
		t.Fatal("inventory category changed the published sell price")
	}
	for k := TraderKind(0); k < NumKinds; k++ {
		if Wants(b, k, dust) || Price(b, dust, k, Buy, Terms{}) != 0 || Wants(b, k, item.None) {
			t.Fatalf("trader %d wants or prices a value-0 or gone kind", k)
		}
	}
	// selling and gifting price the plugin good by the book
	st := Stock{Kind: CurioMerchant, Silver: 1000}
	if r := Settle(b, []Delta{{Kind: bell, Qty: -3}}, map[item.Kind]int{bell: 3}, st, Terms{}, false); !r.OK() || r.SilverCenti != 3*1080 {
		t.Fatalf("selling 3 bells: %+v", r)
	}
	if r := Settle(b, []Delta{{Kind: bell, Qty: -3}}, map[item.Kind]int{bell: 3}, Stock{Kind: BulkHauler, Silver: 1000}, Terms{}, false); r.Reject != RejectUnwanted {
		t.Fatalf("the bulk hauler took bells: %+v", r)
	}
	if v := GiftValue(b, []Delta{{Kind: bell, Qty: -3}, {Kind: item.Silver, Qty: -10}}, Terms{FactionType: faction.Settlers}); v != 42 {
		t.Fatalf("gift value %d, want 42", v)
	}
	if r := Settle(b, []Delta{{Kind: dust, Qty: -1}}, map[item.Kind]int{dust: 1}, st, Terms{}, true); r.Reject != RejectUnwanted {
		t.Fatalf("a value-0 gift: %+v", r)
	}
	if r := Settle(b, []Delta{{Kind: bell, Qty: -2}}, map[item.Kind]int{bell: 2}, st, Terms{FactionType: faction.Settlers}, true); !r.OK() || r.Goodwill != faction.GiftGoodwill(faction.Faction{Type: faction.Settlers}, GiftValue(b, []Delta{{Kind: bell, Qty: -2}}, Terms{FactionType: faction.Settlers})) {
		t.Fatalf("a gift of 2 bells: %+v", r)
	}
}

// TestGoodsNameParseOfferRoundTrip: a plugin good is named by its key and
// labelled by its def; the name parses back, the crate form still parses,
// and an offer may name any kind of the run's book but none past it.
func TestGoodsNameParseOfferRoundTrip(t *testing.T) {
	t.Parallel()
	b := testBook(item.Def{Label: "copper bell", Path: "Manufactured/Components", Value: 40})
	bell := Goods{Kind: item.Kind(item.CoreKinds)}
	if bell.Name(b) != "test.p0" || bell.Label(b) != "copper bell" || bell.Value(b) != 40 {
		t.Fatalf("name %q label %q value %d", bell.Name(b), bell.Label(b), bell.Value(b))
	}
	if g, ok := ParseGoods(b, bell.Name(b)); !ok || g != bell {
		t.Fatalf("parse %q: %+v %v", bell.Name(b), g, ok)
	}
	if _, ok := ParseGoods(item.Book{}, "test.p0"); ok {
		t.Fatal("the core book parses a plugin key")
	}
	// core goods keep their names and the caller's label
	steel := Goods{Kind: item.Steel}
	if steel.Name(b) != "steel" || steel.Label(b) != "" {
		t.Fatalf("steel: name %q label %q", steel.Name(b), steel.Label(b))
	}
	if g, ok := ParseGoods(b, "steel"); !ok || g != steel {
		t.Fatalf("parse steel: %+v %v", g, ok)
	}
	tv := Goods{item.MinifiedBuilding, 0}
	for _, l := range Generate(b, CurioMerchant, faction.Spacer, randx.New(3)).Lines {
		if l.Building != 0 {
			tv = l.Goods()
		}
	}
	if g, ok := ParseGoods(b, tv.Name(b)); tv.Building == 0 || !ok || g != tv {
		t.Fatalf("crate %+v parses to %+v %v", tv, g, ok)
	}
	offer := []Delta{{Kind: item.Steel, Qty: 2}, {Kind: bell.Kind, Qty: -5}}
	got, err := DecodeOffer(b, EncodeOffer(offer))
	if err != nil || len(got) != 2 || got[0] != offer[0] || got[1] != offer[1] {
		t.Fatalf("offer round trip %+v %v", got, err)
	}
	for _, k := range []item.Kind{item.Kind(b.Len()), item.None} {
		if _, err := DecodeOffer(b, EncodeOffer([]Delta{{Kind: k, Qty: 1}})); err == nil {
			t.Fatalf("kind %d past the book decodes", k)
		}
	}
	if _, err := DecodeOffer(item.Book{}, EncodeOffer([]Delta{{Kind: bell.Kind, Qty: 1}})); err == nil {
		t.Fatal("the core book decodes a plugin kind")
	}
}

// TestGenerateStocksOptIns: a trader that wants an opted-in plugin kind
// stocks 1..TraderStock of it on some visits and not on others; its core
// stock and silver are what the core book draws; a trader that does not
// want it, or a kind with no opt-in, draws nothing more.
func TestGenerateStocksOptIns(t *testing.T) {
	t.Parallel()
	b := testBook(
		item.Def{Label: "bell", Path: "Manufactured/Components", Value: 40, TraderStock: 6},
		item.Def{Label: "ore", Path: "Raw resources/Metals", Value: 7, TraderStock: 30},
		item.Def{Label: "plain", Path: "Manufactured/Components", Value: 40},
		item.Def{Label: "dust", Path: "Manufactured/Components", Value: 0, TraderStock: 9},
	)
	bell, ore := item.Kind(item.CoreKinds), item.Kind(item.CoreKinds+1)
	wants := map[TraderKind]item.Kind{CurioMerchant: bell, BulkHauler: ore}
	for k := TraderKind(0); k < NumKinds; k++ {
		with, without := 0, 0
		for seed := uint64(1); seed <= 50; seed++ {
			got, core := Generate(b, k, faction.Industrial, randx.New(seed)), Generate(item.Book{}, k, faction.Industrial, randx.New(seed))
			var rest []Line
			for _, l := range got.Lines {
				if !b.Plugin(l.Kind) {
					rest = append(rest, l)
					continue
				}
				if want, ok := wants[k]; !ok || l.Kind != want || l.Qty < 1 || l.Qty > b.Def(l.Kind).TraderStock {
					t.Fatalf("trader %d seed %d stocks %+v", k, seed, l)
				}
				with++
			}
			if got.Silver != core.Silver || fmt.Sprint(rest) != fmt.Sprint(core.Lines) {
				t.Fatalf("trader %d seed %d: core stock %+v, want %+v", k, seed, rest, core.Lines)
			}
			if len(rest) == len(got.Lines) {
				without++
			}
		}
		if _, ok := wants[k]; ok && (with == 0 || without == 0) {
			t.Fatalf("trader %d stocks its plugin good on %d of 50 visits", k, with)
		}
	}
	// the quartermaster wants none of them: its stream is the core one
	for _, tech := range []faction.Tech{faction.Neolithic, faction.Industrial, faction.Spacer} {
		key := fmt.Sprintf("%d/%d", Quartermaster, tech)
		if got := streamHash(b, Quartermaster, tech); got != coreStream[key] {
			t.Fatalf("quartermaster %s: stream %q, want %q", key, got, coreStream[key])
		}
	}
	if a, c := Generate(b, CurioMerchant, faction.Spacer, randx.New(9)), Generate(b, CurioMerchant, faction.Spacer, randx.New(9)); fmt.Sprint(a) != fmt.Sprint(c) {
		t.Fatal("the same seed stocks differently")
	}
}

// New medical devices must actually appear in generated core stock without
// modifying the seeded pre-MED merchant goods or the caller RNG.
func TestCurioMerchantStocksCoreMedicalImplants(t *testing.T) {
	t.Parallel()
	seen := make(map[item.Kind]bool)
	for seed := uint64(1); seed <= 250; seed++ {
		stock := Generate(item.Book{}, CurioMerchant, faction.Spacer, randx.New(seed))
		for _, l := range stock.Lines {
			if medicalAdded(l.Kind) {
				if l.Qty != 1 {
					t.Fatalf("seed %d: MED-14 implant stock %+v", seed, l)
				}
				seen[l.Kind] = true
			}
		}
	}
	for _, kind := range [...]item.Kind{
		item.BlissCircuit, item.NerveDampener, item.AcidFailsafe,
		item.AcousticImplant, item.ServoClaw, item.CochlearImplant,
		item.ServoJaw, item.SynthTongue, item.ApexArm, item.ApexLeg,
		item.ApexEye,
	} {
		if !seen[kind] {
			t.Errorf("no generated curio merchant stocked MED-14 %v", kind)
		}
	}
}
