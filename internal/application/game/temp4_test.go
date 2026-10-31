package game

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/entity"
)

// TestNakedThought (TEMP-4, wiki: Naked -6): a new colonist arrives in a
// T-shirt and pants; without torso or leg cover on any layer it feels
// naked; tribalwear alone covers both.
func TestNakedThought(t *testing.T) {
	t.Parallel()
	g, id := dressGame(t, 12, 8, 180)
	if wears(g, id, item.SkinTorso) != item.TShirt || wears(g, id, item.SkinLegs) != item.Pants || hasThought(g, id, mood.Naked, -6) {
		t.Fatalf("a new colonist wears %v, thoughts %v", g.Wearing(id), g.thoughts(id))
	}
	w := g.apparel[id]
	w.On[item.SkinLegs] = craft.Piece{}
	g.apparel[id] = w
	if !hasThought(g, id, mood.Naked, -6) {
		t.Fatalf("no pants: %v", g.thoughts(id))
	}
	w.On[item.SkinTorso] = craft.Piece{Kind: item.Tribalwear}
	g.apparel[id] = w
	if hasThought(g, id, mood.Naked, -6) {
		t.Fatal("tribalwear covers torso and legs")
	}
	if pv := pawnView(g, id); len(pv.Apparel) != 1 || pv.Apparel[0] != "tribalwear" {
		t.Fatalf("view %v", pv.Apparel)
	}
}

// TestGeneratedColonistsDress (TEMP-4): a generated colonist wears a
// T-shirt and pants, a tribal one tribalwear, and one arriving with no
// clothes (NoClothes) nothing, and it feels naked.
func TestGeneratedColonistsDress(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	var tribal backstory.ID
	for _, d := range backstory.All() {
		if d.Slot == backstory.Childhood && d.Tags == backstory.TagNeolithic {
			tribal = d.ID
			break
		}
	}
	if tribal == 0 {
		t.Fatal("setup: no tribal childhood")
	}
	base := pawngen.Legacy(1, "Ada", 1, 1, 1)
	plain, _ := g.spawnGenerated(base, 2, 2)
	p := base
	p.Childhood = tribal
	tr, _ := g.spawnGenerated(p, 3, 2)
	p = base
	p.Look.Bare = true
	bare, _ := g.spawnGenerated(p, 4, 2)
	if wears(g, plain, item.SkinTorso) != item.TShirt || wears(g, plain, item.SkinLegs) != item.Pants {
		t.Fatalf("plain colonist wears %v", g.Wearing(plain))
	}
	if wears(g, tr, item.SkinTorso) != item.Tribalwear || wears(g, tr, item.SkinLegs) != 0 {
		t.Fatalf("tribal colonist wears %v", g.Wearing(tr))
	}
	if len(g.Wearing(bare)) != 0 || !hasThought(g, bare, mood.Naked, -6) {
		t.Fatalf("bare colonist wears %v, thoughts %v", g.Wearing(bare), g.thoughts(bare))
	}
}

// TestOldSaveColonistsGetEverydayClothes (TEMP-4, save v8): a colonist of
// a save from before the layers wore its everyday clothes implicitly, so
// it loads in a T-shirt and pants (over what it had on); a v8 save keeps
// a naked colonist naked, and the clothes round-trip.
func TestOldSaveColonistsGetEverydayClothes(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	bo, _ := g.SpawnNamedPawn(2, 1, "Bo")
	delete(g.apparel, ada) // an older save: no outfit at all
	var w craft.Outfit
	w.On[item.Body] = craft.Piece{Kind: item.Parka}
	g.apparel[bo] = w
	l := loadAsVersion(t, g, 7)
	for _, id := range []entity.EntityID{ada, bo} {
		if wears(l, id, item.SkinTorso) != item.TShirt || wears(l, id, item.SkinLegs) != item.Pants || l.apparel[id].Naked() {
			t.Fatalf("pawn %d loaded in %v", id, l.Wearing(id))
		}
	}
	if wears(l, bo, item.Body) != item.Parka {
		t.Fatal("the parka stays on")
	}
	h := saveLoad(t, g) // v8: what is on, as it is
	if len(h.Wearing(ada)) != 0 || wears(h, bo, item.SkinTorso) != 0 || wears(h, bo, item.Body) != item.Parka {
		t.Fatalf("v8 load: %v, %v", h.Wearing(ada), h.Wearing(bo))
	}
	if again := saveLoad(t, l); again.StateHash() != l.StateHash() {
		t.Fatal("a second load changed the game")
	}
}

// TestTailorSewsTShirt (TEMP-4): the tailoring bench sews a T-shirt from
// 4 cloth (wiki 40 / 10) with a quality, and a tribal colonist can make
// tribalwear at a crafting spot.
func TestTailorSewsTShirt(t *testing.T) {
	t.Parallel()
	d := recipe.DefOf(recipe.MakeTShirt)
	if d.Input.Qty != 4 || d.Work != 160 || d.Output != item.TShirt || !d.Stuffed() || !recipe.DefOf(recipe.MakeTribalwear).At(construction.CraftingSpot) {
		t.Fatalf("recipe %+v", d)
	}
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	ada, _ := g.SpawnNamedPawn(5, 5, "Ada")
	onlyWork(g, ada, work.Craft)
	placeBuilt(t, g, construction.TailoringBench, construction.South, 15, 2)
	g.Enqueue(commandAddBill(15, 2, recipe.MakeTShirt))
	g.SpawnStack(item.Cloth, 4, 8, 8)
	for i := 0; i < 100 && itemsOnGround(g, "t-shirt") == 0; i++ {
		g.Step(50)
		g.SetNeeds(ada, needs.Full, needs.Full)
	}
	st, ok := stackOf(g, item.TShirt)
	if !ok || st.StuffOfStack() != item.Cloth || st.Quality == 0 || itemsOnGround(g, "cloth") != 0 {
		t.Fatalf("sewn %+v (ok %v), cloth left %d", st, ok, itemsOnGround(g, "cloth"))
	}
}
