package craft

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

func TestPieceRoundTrip(t *testing.T) {
	t.Parallel()
	st := item.Stack{Kind: item.Parka, Qty: 3, Stuff: item.Cloth, Quality: quality.Stored(quality.Good), Wear: 12, Of: 4}
	p := PieceOf(st)
	back := p.Stack(5, 6)
	if back.Kind != st.Kind || back.Qty != 1 || back.Stuff != st.Stuff || back.Quality != st.Quality || back.Wear != 12 || back.Of != 4 || back.X != 5 {
		t.Fatalf("round trip %+v", back)
	}
	if p.MaxHP() != 140 || p.HP() != 128 {
		t.Fatalf("hp %d/%d", p.HP(), p.MaxHP())
	}
}

func TestOutfitSlotsForcedTaint(t *testing.T) {
	t.Parallel()
	var o Outfit
	o.On[item.Head] = Piece{Kind: item.Tuque}
	if o.Kind(item.Head) != item.Tuque || o.Kind(item.NoSlot) != 0 || o.Kind(item.Slot(9)) != 0 {
		t.Fatal("Kind")
	}
	o.SetForced(item.Head, true)
	if !o.IsForced(item.Head) || o.IsForced(item.Body) {
		t.Fatal("forced")
	}
	o.SetForced(item.Head, false)
	if o.Forced != 0 {
		t.Fatal("unforced")
	}
	tainted := o.Taint(9)
	if tainted.On[item.Head].Of != 9 || tainted.On[item.Body].Of != 0 || o.On[item.Head].Of != 0 {
		t.Fatal("Taint marks the worn pieces of a copy")
	}
}

func TestOutfitSaveAndLegacy(t *testing.T) {
	t.Parallel()
	var o Outfit
	o.On[item.Body] = Piece{Kind: item.Duster, Stuff: item.Cloth, Quality: 5, Wear: 3}
	o.SetForced(item.Body, true)
	m := map[entity.EntityID]Outfit{1: o}
	if got := ImportOutfits(ExportOutfits(m), nil); got[1] != o {
		t.Fatalf("round trip %+v", got[1])
	}
	if ExportOutfits(nil) != nil {
		t.Fatal("no outfits save as nothing")
	}
	legacy := map[entity.EntityID][3]item.Kind{2: {1 << item.Head, item.Parka, item.Tuque}}
	got := ImportOutfits(nil, legacy)[2]
	if got.Kind(item.Body) != item.Parka || got.Kind(item.Head) != item.Tuque || !got.IsForced(item.Head) || got.IsForced(item.Body) {
		t.Fatalf("legacy %+v", got)
	}
	if got := ImportOutfits(map[entity.EntityID]OutfitSave{3: {On: []Piece{{}, {Kind: item.Parka}}}}, legacy); len(got) != 1 || got[3].Kind(item.Body) != item.Parka {
		t.Fatalf("the new field wins: %+v", got)
	}
	if got := ImportOutfits(nil, nil); got == nil || len(got) != 0 {
		t.Fatal("empty, never nil")
	}
}

func TestOutfitThoughts(t *testing.T) {
	t.Parallel()
	var o Outfit
	if ts := o.AppendThoughts(nil); ts != nil {
		t.Fatal("bare: no thoughts")
	}
	o.On[item.Body] = Piece{Kind: item.Duster, Stuff: item.Cloth} // 200 HP
	if ts := o.AppendThoughts(nil); ts != nil {
		t.Fatal("fine clothes: no thoughts")
	}
	o.On[item.Body].Wear = 100 // 50 %: ratty
	ts := o.AppendThoughts(nil)
	if len(ts) != 1 || ts[0].Kind != mood.RattyApparel || ts[0].Value != -3 {
		t.Fatalf("ratty %v", ts)
	}
	o.On[item.Head] = Piece{Kind: item.Tuque, Stuff: item.Cloth, Wear: 70} // 10 of 80: tattered
	ts = o.AppendThoughts(nil)
	if len(ts) != 1 || ts[0].Kind != mood.TatteredApparel || ts[0].Value != -5 {
		t.Fatalf("tattered (the worst piece) %v", ts)
	}
	o = o.Taint(7)
	ts = o.AppendThoughts(nil)
	if len(ts) != 2 || ts[1].Kind != mood.TaintedApparel || ts[1].Value != -8 {
		t.Fatalf("two tainted %v", ts)
	}
	for n, want := range map[int]int{0: 0, 1: -5, 3: -11, 4: -14, 9: -14} {
		if TaintedMood(n) != want {
			t.Errorf("TaintedMood(%d) = %d", n, TaintedMood(n))
		}
	}
}

func TestOutfitViewsAndBands(t *testing.T) {
	t.Parallel()
	var o Outfit
	if o.Views() != nil || o.Bands() != exposure.Everyday {
		t.Fatal("bare")
	}
	o.On[item.Body] = Piece{Kind: item.Parka, Quality: quality.Stored(quality.Masterwork), Wear: 150, Of: 3}
	v := o.Views()
	if len(v) != 1 || v[0].Kind != "parka" || v[0].Stuff != "leather" || v[0].Quality != "masterwork" || v[0].HP != 32 || v[0].MaxHP != 182 ||
		!v[0].Tainted || !v[0].Tattered || v[0].Ratty {
		t.Fatalf("view %+v", v)
	}
	plain := Outfit{}
	plain.On[item.Body] = Piece{Kind: item.Parka}
	if o.Bands().Min >= plain.Bands().Min {
		t.Fatal("a masterwork parka keeps warmer than a plain one")
	}
	if QualityLabel(0) != "" || StuffLabel(item.Rifle, item.Steel) != "" || StuffLabel(item.Club, 0) != "wood" {
		t.Fatal("labels")
	}
	b := exposure.Bands{Min: 0, Max: 300}
	if DayFit(b, 150) != 0 || DayFit(b, 0) != 100 || DayFit(b, 400) != 200 || DaySurplus(b, 150) != 100 {
		t.Fatal("day fit")
	}
}
