package outfit

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

type fakeWearers []entity.EntityID

func (f fakeWearers) Wearer(id entity.EntityID) bool { return slices.Contains(f, id) }
func (f fakeWearers) Wearers() []entity.EntityID     { return f }

func garment(k item.Kind) item.Stack { return item.Stack{Kind: k, Qty: 1} }

// worn is k with pct percent of its hit points left.
func worn(k item.Kind, pct int) item.Stack {
	st := garment(k)
	st.Wear = st.MaxHP() - st.MaxHP()*pct/100
	return st
}

func TestDefaultPoliciesFilter(t *testing.T) {
	t.Parallel()
	ps := Defaults()
	tainted := garment(item.Parka)
	tainted.Of = 7
	for _, c := range []struct {
		pol  int
		st   item.Stack
		want bool
	}{
		{Anything, garment(item.Parka), true},
		{Anything, garment(item.FlakVest), true},
		{Anything, worn(item.Parka, 10), true},
		{Anything, tainted, false},
		{Worker, garment(item.Parka), true},
		{Worker, garment(item.FlakVest), false},
		{Worker, garment(item.SimpleHelmet), false},
		{Worker, worn(item.Parka, 40), false},
		{Worker, worn(item.Parka, 60), true},
		{Soldier, garment(item.FlakVest), true},
		{Soldier, garment(item.AdvancedHelmet), true},
		{Soldier, worn(item.FlakVest, 40), false},
		{Nudist, garment(item.TShirt), false},
		{Nudist, garment(item.ShieldBelt), true},
		{Nudist, garment(item.Wood), true}, // not apparel: not a policy's to forbid
	} {
		if got := ps[c.pol].Takes(c.st); got != c.want {
			t.Errorf("%s takes %v (%d/%d hp, of %d): %v, want %v", ps[c.pol].Name, c.st.Kind, c.st.HP(), c.st.MaxHP(), c.st.Of, got, c.want)
		}
	}
}

func TestEditPolicyFilter(t *testing.T) {
	t.Parallel()
	var b Book
	w := fakeWearers{1}
	edit := func(mode uint8, name string, on bool, x, y int) bool {
		return b.Apply(command.Command{Kind: command.EditApparelPolicy, Index: Anything, Mode: mode, Name: name, Enabled: on, X: x, Y: y}, w)
	}
	leather, cloth := garment(item.Parka), garment(item.Parka)
	leather.Stuff, cloth.Stuff = item.Leather, item.Cloth
	if !edit(command.ApparelPolicyStuff, "leather", false, 0, 0) {
		t.Fatal("forbidding leather changed nothing")
	}
	if p := b.Of(1); p.Takes(leather) || !p.Takes(cloth) || p.Takes(garment(item.Parka)) { // a parka is leather unless it says otherwise
		t.Fatalf("leather forbidden: leather %v, cloth %v", p.Takes(leather), p.Takes(cloth))
	}
	if !edit(command.ApparelPolicyAllow, "Apparel/Armor", false, 0, 0) || b.Of(1).Takes(garment(item.FlakVest)) {
		t.Fatal("armour still allowed")
	}
	if !edit(command.ApparelPolicyAllow, "flak vest", true, 0, 0) || !b.Of(1).Takes(garment(item.FlakVest)) || b.Of(1).Takes(garment(item.PlateArmor)) {
		t.Fatal("one kind back")
	}
	if edit(command.ApparelPolicyAllow, "wood", true, 0, 0) || edit(command.ApparelPolicyStuff, "parka", false, 0, 0) {
		t.Fatal("a kind outside Apparel, or a non-material, edits a policy")
	}
	good := cloth
	good.Quality = quality.Stored(quality.Good)
	poor := good
	poor.Quality = quality.Stored(quality.Poor)
	if !edit(command.ApparelPolicyQuality, "", false, int(quality.Normal), int(quality.Legendary)) || b.Of(1).Takes(poor) || !b.Of(1).Takes(good) {
		t.Fatal("quality range")
	}
	tainted := garment(item.TShirt)
	tainted.Of = 3
	if !edit(command.ApparelPolicyTainted, "", true, 0, 0) || !b.Of(1).Takes(tainted) {
		t.Fatal("tainted allowed")
	}
	v := b.Views()[Anything]
	if slices.Contains(v.Stuffs, "leather") || !slices.Contains(v.Stuffs, "cloth") || !slices.Contains(v.Allowed, "flak vest") ||
		slices.Contains(v.Allowed, "plate armor") || v.NoTainted || v.QualityMin != uint8(quality.Normal) || !v.Default {
		t.Fatalf("view %+v", v)
	}
}

// Deleting a policy moves its holders to the default; the default and the
// last policy can't be deleted; a colonist always has a policy.
func TestDeletePolicyReassigns(t *testing.T) {
	t.Parallel()
	var b Book
	w := fakeWearers{1, 2}
	apply := func(c command.Command) bool { return b.Apply(c, w) }
	if !apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyCreate, Index: Worker, Name: "Miner"}) {
		t.Fatal("create")
	}
	miner := b.Policies()[len(b.Policies())-1]
	if miner.ID != numDefaults || miner.Name != "Miner" || miner.Allow != Defaults()[Worker].Allow {
		t.Fatalf("copy of Worker: %+v", miner)
	}
	if !apply(command.Command{Kind: command.SetApparelPolicy, Pawn: 1, Index: miner.ID}) || b.Of(1).Name != "Miner" {
		t.Fatal("assign")
	}
	if apply(command.Command{Kind: command.SetApparelPolicy, Pawn: 9, Index: miner.ID}) {
		t.Fatal("a non-colonist holds a policy")
	}
	if !apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDelete, Index: miner.ID}) {
		t.Fatal("delete")
	}
	if b.Of(1).ID != Anything || len(b.Policies()) != numDefaults {
		t.Fatalf("after delete: %v on %q", len(b.Policies()), b.Of(1).Name)
	}
	if apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDelete, Index: Anything}) {
		t.Fatal("deleted the default")
	}
	for _, id := range []int{Worker, Soldier, Nudist} {
		apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDelete, Index: id})
	}
	if len(b.Policies()) != 1 || b.Of(2).ID != Anything {
		t.Fatalf("left %v", b.Policies())
	}
}

// A new default is for new colonists: those already on the old one keep it.
func TestDefaultPolicyIsForNewColonists(t *testing.T) {
	t.Parallel()
	var b Book
	w := fakeWearers{1, 2}
	b.Apply(command.Command{Kind: command.SetApparelPolicy, Pawn: 2, Index: Soldier}, w)
	if !b.Apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDefault, Index: Worker}, w) {
		t.Fatal("set default")
	}
	if b.Of(1).ID != Anything || b.Of(2).ID != Soldier || b.Of(3).ID != Worker || b.Default() != Worker {
		t.Fatalf("1 %q, 2 %q, new 3 %q", b.Of(1).Name, b.Of(2).Name, b.Of(3).Name)
	}
	// deleting a policy now moves its holders to Worker
	b.Apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDelete, Index: Soldier}, w)
	if b.Of(2).ID != Worker {
		t.Fatalf("2 on %q", b.Of(2).Name)
	}
	if v := b.Views(); v[Worker].Default != true || v[Anything].Default {
		t.Fatalf("views %+v", v)
	}
}

func TestBookExportImport(t *testing.T) {
	t.Parallel()
	var b Book
	if b.Export() != nil {
		t.Fatal("an untouched book saves nothing")
	}
	w := fakeWearers{1, 2}
	b.Apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyCreate, Index: -1, Name: "Parade"}, w)
	b.Apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyRename, Index: Nudist, Name: "Bare"}, w)
	b.Apply(command.Command{Kind: command.SetApparelPolicy, Pawn: 2, Index: numDefaults}, w)
	b.Apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyDefault, Index: Soldier}, w)
	var c Book
	c.Import(b.Export())
	if len(c.Views()) != 5 || c.Default() != Soldier ||
		c.Views()[Nudist].Name != "Bare" || c.Of(2).Name != "Parade" || c.Of(1).ID != Anything || c.Of(5).ID != Soldier {
		t.Fatalf("after import: %+v; 1 %q, 2 %q", c.Views(), c.Of(1).Name, c.Of(2).Name)
	}
	c.Apply(command.Command{Kind: command.EditApparelPolicy, Mode: command.ApparelPolicyCreate, Index: -1, Name: "Next"}, w)
	if c.Policies()[5].ID != numDefaults+1 {
		t.Fatal("ids are not reused after a load")
	}
	c.Import(nil)
	if c.Export() != nil || c.Of(2).ID != Anything {
		t.Fatal("a save without outfits loads the defaults")
	}
}
