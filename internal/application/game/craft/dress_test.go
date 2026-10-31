package craft

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/entity"
)

// wikiBase is the wiki's bare human range, 16..26 C (TEMP-3 sets
// exposure.Everyday to it; the search takes its base as an argument).
var wikiBase = exposure.Bands{Min: 160, Max: 260}

// TestOutfitLayers (TEMP-4): naked is no torso or no leg cover on any
// layer; conflicts are the same layer over the same region; everyday
// clothes fill free skin slots; the tribal start wears tribalwear.
func TestOutfitLayers(t *testing.T) {
	t.Parallel()
	var o Outfit
	if !o.Naked() {
		t.Fatal("bare is naked")
	}
	o.On[item.Body] = Piece{Kind: item.Parka}
	if !o.Naked() {
		t.Fatal("a parka leaves the legs bare")
	}
	o.On[item.Legs] = Piece{Kind: item.FlakPants}
	if o.Naked() {
		t.Fatal("a parka and flak pants cover torso and legs")
	}
	d := Outfit{}.Everyday()
	if d.Kind(item.SkinTorso) != item.TShirt || d.Kind(item.SkinLegs) != item.Pants || d.Naked() {
		t.Fatalf("everyday %+v", d)
	}
	if got := d.Conflicts(item.Tribalwear); got != 1<<item.SkinTorso|1<<item.SkinLegs {
		t.Fatalf("tribalwear displaces %b", got)
	}
	if got := d.Conflicts(item.Parka); got != 0 {
		t.Fatalf("a parka goes over the shirt: %b", got)
	}
	d.On[item.Middle] = Piece{Kind: item.FlakVest}
	if got := d.Conflicts(item.MarineArmor); got != 1<<item.Middle {
		t.Fatalf("marine armor displaces %b", got)
	}
	tr := Tribal()
	if tr.Naked() || tr.Everyday() != tr {
		t.Fatalf("tribal %+v: covered, no pants under it", tr)
	}
	c, h := d.On[item.SkinTorso].Insulation()
	if b := (Outfit{}).Everyday().Bands(); b.Min != exposure.Everyday.Min-c-36 || b.Max != exposure.Everyday.Max+h+14 {
		t.Fatalf("bands %+v: every layer counts", b)
	}
}

// TestImportMovesFlakVest (TEMP-4): a five-slot outfit saved before the
// layers wore its flak vest in Body; it loads in Middle with its forced
// bit, and an eight-slot outfit loads as it is.
func TestImportMovesFlakVest(t *testing.T) {
	t.Parallel()
	old := map[entity.EntityID]OutfitSave{1: {On: []Piece{{}, {Kind: item.FlakVest, Wear: 3}, {Kind: item.SimpleHelmet}, {}, {}},
		Forced: 1 << item.Body}}
	o := ImportOutfits(old, nil)[1]
	if o.Kind(item.Body) != 0 || o.Kind(item.Middle) != item.FlakVest || o.On[item.Middle].Wear != 3 ||
		!o.IsForced(item.Middle) || o.IsForced(item.Body) || o.Kind(item.Head) != item.SimpleHelmet {
		t.Fatalf("migrated %+v", o)
	}
	var n Outfit
	n.On[item.Middle] = Piece{Kind: item.FlakVest}
	n.On[item.Body] = Piece{Kind: item.Parka}
	if got := ImportOutfits(ExportOutfits(map[entity.EntityID]Outfit{2: n}), nil)[2]; got != n {
		t.Fatalf("round trip %+v", got)
	}
}

// opts builds Best's per-slot option lists: on is what is worn (keep
// first, then bare), lying the kinds at hand, each walk away.
func opts(on Outfit, walk int, lying ...item.Kind) *[Slots][]Option {
	var o [Slots][]Option
	for s := item.Body; s < item.NumSlots; s++ {
		o[s] = []Option{{Kind: on.Kind(s)}}
		if on.Kind(s) != 0 {
			o[s] = append(o[s], Option{})
		}
	}
	for i, k := range lying {
		o[k.Slot()] = append(o[k.Slot()], Option{Kind: k, Stack: entity.EntityID(i + 1), Walk: walk})
	}
	return &o
}

// TestBestLayers (TEMP-4): the search wears layers together, never two
// garments on one occupied slot, prefers covered to naked, takes nothing
// off just to spare insulation, and keeps what is on in a tie.
func TestBestLayers(t *testing.T) {
	t.Parallel()
	everyday := Outfit{}.Everyday()
	// -17 C: parka and tuque over the shirt and pants.
	p := Best(opts(everyday, 3, item.Parka, item.Tuque, item.Duster), wikiBase, -170)
	if p.Pick[item.Body].Kind != item.Parka || p.Pick[item.Head].Kind != item.Tuque || p.Pick[item.SkinTorso].Kind != item.TShirt ||
		p.Pick[item.SkinLegs].Kind != item.Pants || p.Naked || p.Off != 0 || p.Miss != 0 {
		t.Fatalf("winter plan %+v", p)
	}
	if n := p.Next(); n.Stack == 0 || n.Walk != 3 {
		t.Fatalf("next %+v", n)
	}
	// 2 C: a parka over the T-shirt, which stays on (spared insulation is no reason).
	p = Best(opts(everyday, 1, item.Parka), wikiBase, 20)
	if p.Pick[item.Body].Kind != item.Parka || p.Pick[item.SkinTorso].Kind != item.TShirt || p.Off != 0 {
		t.Fatalf("cold plan %+v", p)
	}
	// Marine armor is a shell: the flak vest and flak pants come off for it.
	worn := everyday
	worn.On[item.Middle], worn.On[item.Legs] = Piece{Kind: item.FlakVest}, Piece{Kind: item.FlakPants}
	p = Best(opts(worn, 1, item.MarineArmor), wikiBase, -250)
	if p.Pick[item.Body].Kind != item.MarineArmor || p.Pick[item.Middle].Kind != 0 || p.Pick[item.Legs].Kind != 0 || p.Off != 2 {
		t.Fatalf("shell plan %+v", p)
	}
	// Naked with a parka at hand on a mild day: pants first, covered wins.
	var bare Outfit
	p = Best(opts(bare, 1, item.Parka, item.Pants), wikiBase, 210)
	if p.Naked || p.Pick[item.SkinLegs].Kind != item.Pants || p.Pick[item.Body].Kind != item.Parka {
		t.Fatalf("naked plan %+v", p)
	}
	// Nothing better: what is on, untouched.
	p = Best(opts(everyday, 1), wikiBase, 180)
	if p.Pick[item.SkinTorso].Kind != item.TShirt || p.Pick[item.SkinLegs].Kind != item.Pants || p.Next().Stack != 0 || p.Off != 0 {
		t.Fatalf("idle plan %+v", p)
	}
}
