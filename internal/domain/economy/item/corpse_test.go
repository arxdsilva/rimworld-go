package item

import "testing"

func TestCorpsesDoNotMergeAcrossSpecies(t *testing.T) {
	t.Parallel()
	deer, wolf := Stack{Kind: CorpseAnimal, Qty: 1, Animal: 0}, Stack{Kind: CorpseAnimal, Qty: 1, Animal: 1}
	if CanMerge(deer, wolf) {
		t.Fatal("a deer and a timber wolf corpse never share a stack")
	}
	if !CanMerge(deer, Stack{Kind: CorpseAnimal, Qty: 2}) {
		t.Fatal("two deer corpses stack")
	}
}

func TestAnimalCorpseKind(t *testing.T) {
	t.Parallel()
	k := CorpseAnimal
	if !k.IsCorpse() || k.IsFood() || k.Dryable() || !k.Perishable() || k.ShelfTicks() != CorpseDustbuck.ShelfTicks() {
		t.Fatalf("an animal corpse rots like the old ones and is butchered, not eaten: %v", k)
	}
	if k.Value() <= 0 || k.String() != "animal corpse" {
		t.Fatalf("value %d name %q", k.Value(), k.String())
	}
	if k, ok := ParseKind("animal corpse"); !ok || k != CorpseAnimal {
		t.Fatal("parse")
	}
}

func TestLegacyCorpsesConvert(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     Stack
		animal uint8
		ok     bool
	}{
		{Stack{Kind: CorpseDustbuck, Qty: 2, X: 3, Rot: 50, Carrier: 9}, 0, true},
		{Stack{Kind: CorpseRidgewolf, Qty: 1, Y: 4}, 1, true},
		{Stack{Kind: Meat, Qty: 1}, 0, false},
		{Stack{Kind: CorpseAnimal, Qty: 1, Animal: 7}, 7, false},
	}
	for _, c := range cases {
		got, ok := ConvertLegacy(c.in)
		if ok != c.ok {
			t.Fatalf("%v: converted %v", c.in.Kind, ok)
		}
		if !ok {
			if got != c.in {
				t.Fatalf("non-legacy stack changed: %+v", got)
			}
			continue
		}
		want := c.in
		want.Kind, want.Animal = CorpseAnimal, c.animal
		if got != want {
			t.Fatalf("got %+v want %+v", got, want)
		}
	}
}
