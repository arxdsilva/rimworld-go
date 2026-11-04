package sense

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeGear is the game's side of Gear: a few stacks and pawns.
type fakeGear struct {
	pawns  []entity.EntityID
	temp   int
	manip  int
	stacks map[entity.EntityID]item.Stack
	next   entity.EntityID
	down   []item.Stack
	rotted int
	noMap  bool
}

func (f *fakeGear) PawnIDs() []entity.EntityID      { return f.pawns }
func (f *fakeGear) Temperature(entity.EntityID) int { return f.temp }
func (f *fakeGear) Manipulation(entity.EntityID) int {
	return f.manip
}
func (f *fakeGear) Rotted(_ entity.EntityID, _ item.Kind, qty int) { f.rotted += qty }
func (f *fakeGear) Stack(s entity.EntityID) (item.Stack, bool) {
	st, ok := f.stacks[s]
	return st, ok
}
func (f *fakeGear) RemoveQty(s entity.EntityID, n int) {
	st := f.stacks[s]
	if st.Qty -= n; st.Qty <= 0 {
		delete(f.stacks, s)
		return
	}
	f.stacks[s] = st
}
func (f *fakeGear) SpawnCarried(id entity.EntityID, st item.Stack) (entity.EntityID, bool) {
	if f.noMap {
		return 0, false
	}
	f.next++
	st.Carrier = id
	f.stacks[f.next+1000] = st
	return f.next + 1000, true
}
func (f *fakeGear) SetDown(_ entity.EntityID, st item.Stack) { f.down = append(f.down, st) }

func gearScene() (*Sense, *fakeGear) {
	f := &fakeGear{pawns: []entity.EntityID{1, 2}, temp: 210, manip: 1000, stacks: map[entity.EntityID]item.Stack{
		10: {Kind: item.Steel, Qty: 50},
		11: {Kind: item.Meal, Qty: 3},
		12: {Kind: item.CorpseHuman, Qty: 1},
	}}
	s, _ := errandScene(f)
	return s, f
}

func TestCapacityFollowsManipulation(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	if got := s.Gear.Capacity(1); got != 40000 {
		t.Fatalf("whole: %d g", got)
	}
	f.manip = 500
	if got := s.Gear.Capacity(1); got != 20000 {
		t.Fatalf("half: %d g", got)
	}
	f.manip = 0
	if s.Gear.Capacity(1) != 0 || s.Gear.Room(1) != 0 || s.Gear.Stow(1, 10, 1) {
		t.Fatal("no hands, no pack")
	}
}

func TestStowRefusesWhatDoesNotFit(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	limit := s.Gear.Capacity(1) / item.Steel.MassGrams()
	f.stacks[10] = item.Stack{Kind: item.Steel, Qty: limit + 5}
	if s.Gear.Stow(1, 10, limit+1) {
		t.Fatal("stowed past the limit")
	}
	if f.stacks[10].Qty != limit+5 || s.Gear.Has(1) {
		t.Fatal("a refused stow changed something")
	}
	if !s.Gear.Stow(1, 10, limit) || f.stacks[10].Qty != 5 {
		t.Fatalf("up to the limit: %+v", f.stacks[10])
	}
	if s.Gear.Room(1) >= item.Steel.MassGrams() {
		t.Fatalf("room left: %d", s.Gear.Room(1))
	}
	s.Rules = s.Rules.With(gameplay.RespectCapacity, false)
	if !s.Gear.Stow(1, 10, 5) {
		t.Fatal("with the rule off a pack takes anything")
	}
}

func TestStowRefuses(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	f.stacks[13] = item.Stack{Kind: item.Wood, Qty: 1, Carrier: 2}
	for name, try := range map[string]func() bool{
		"no stack":     func() bool { return s.Gear.Stow(1, 99, 1) },
		"nothing":      func() bool { return s.Gear.Stow(1, 11, 0) },
		"more":         func() bool { return s.Gear.Stow(1, 11, 4) },
		"a corpse":     func() bool { return s.Gear.Stow(1, 12, 1) },
		"other's hand": func() bool { return s.Gear.Stow(1, 13, 1) },
	} {
		if try() {
			t.Errorf("%s: stowed", name)
		}
	}
	if s.Gear.Has(1) {
		t.Fatal("something went in")
	}
}

func TestTakeOutFailsWhole(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	s.Gear.Stow(1, 11, 3)
	entry := s.Packs[1].Items[0].ID
	f.noMap = true
	if _, ok := s.Gear.TakeOut(1, entry, 3); ok {
		t.Fatal("no map, no hands")
	}
	if p := s.Packs[1]; len(p.Items) != 1 || p.Items[0].Stack.Qty != 3 || p.Items[0].ID != entry || len(s.FromPack) != 0 {
		t.Fatalf("a failed take changed the pack: %+v", p)
	}
	f.noMap = false
	if _, ok := s.Gear.TakeOut(1, entry+1, 3); ok {
		t.Fatal("no such entry")
	}
	got, ok := s.Gear.TakeOut(1, entry, 2)
	if !ok || f.stacks[got].Qty != 2 || f.stacks[got].Carrier != 1 || s.FromPack[1] != got || s.Packs[1].Items[0].Stack.Qty != 1 {
		t.Fatalf("take: %v %+v, pack %+v", ok, f.stacks[got], s.Packs[1])
	}
}

func TestUnstowIsOneHandLoad(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	f.manip = 100000
	f.stacks[10] = item.Stack{Kind: item.Steel, Qty: item.CarryLimit + 7}
	s.Gear.Stow(1, 10, item.CarryLimit+7)
	got, ok := s.Gear.Unstow(1, item.Steel)
	if !ok || f.stacks[got].Qty != item.CarryLimit || s.Packs[1].Items[0].Stack.Qty != 7 {
		t.Fatalf("unstow: %v %+v, %+v", ok, f.stacks[got], s.Packs[1])
	}
	if _, ok := s.Gear.Unstow(1, item.Wood); ok {
		t.Fatal("unstowed what is not there")
	}
}

func TestSpillClearsThePawn(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	s.Gear.Stow(1, 10, 2)
	s.Gear.Stow(1, 11, 1)
	s.FromPack[1] = 44
	s.Gear.Spill(1)
	if len(f.down) != 2 || f.down[0].Kind != item.Steel || f.down[1].Kind != item.Meal {
		t.Fatalf("spilled out of order: %+v", f.down)
	}
	if len(s.Packs)+len(s.FromPack) != 0 || s.Export() != nil {
		t.Fatalf("left behind: %+v %+v", s.Packs, s.FromPack)
	}
	s.Gear.Spill(1) // nothing left: nothing happens
	if len(f.down) != 2 {
		t.Fatal("spilled twice")
	}
}

// TestSpillNeedsNoRoleWithoutAPack: every removal site calls Spill, also where
// no game side is wired (New(Roles{})), and with no pack that must be nothing.
func TestSpillNeedsNoRoleWithoutAPack(t *testing.T) {
	t.Parallel()
	s := New(Roles{})
	s.Gear.Spill(1)
	s.Gear.Rot(250)
	if s.Gear.Wealth() != 0 || s.Gear.Has(1) {
		t.Fatal("an empty sense has something")
	}
}

func TestPackRotFollowsTheAir(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	s.Gear.Stow(1, 11, 3)
	s.Gear.Stow(1, 10, 1)

	f.temp = -50 // frozen
	s.Gear.Rot(250)
	if got := s.Packs[1].Items[0].Stack.Rot; got != 0 {
		t.Fatalf("frozen food rotted: %d", got)
	}
	f.temp = 50 // half speed
	s.Gear.Rot(250)
	if got := s.Packs[1].Items[0].Stack.Rot; got != 125 {
		t.Fatalf("cool food: rot %d, want 125", got)
	}
	f.temp = 210
	s.Gear.Rot(250)
	if got := s.Packs[1].Items[0].Stack.Rot; got != 375 {
		t.Fatalf("warm food: rot %d, want 375", got)
	}
	if got := s.Packs[1].Items[1].Stack.Rot; got != 0 {
		t.Fatalf("steel rotted: %d", got)
	}
	s.Gear.Rot(s.Packs[1].Items[0].Stack.Shelf())
	if p := s.Packs[1]; len(p.Items) != 1 || p.Items[0].Stack.Kind != item.Steel || f.rotted != 3 {
		t.Fatalf("after spoiling: %+v, reported %d", p, f.rotted)
	}
}

func TestPackRotSkipsPawnsOffTheMap(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	s.Gear.Stow(1, 11, 3)
	f.pawns = []entity.EntityID{2} // 1 is away
	s.Gear.Rot(250)
	if got := s.Packs[1].Items[0].Stack.Rot; got != 0 {
		t.Fatalf("a pack on the road rotted: %d", got)
	}
}

func TestGearWealthCountsThisMap(t *testing.T) {
	t.Parallel()
	s, f := gearScene()
	s.Gear.Stow(1, 10, 2)
	s.Gear.Stow(2, 11, 1)
	want := 2*item.Steel.Value() + item.Meal.Value()
	if got := s.Gear.Wealth(); got != want {
		t.Fatalf("wealth %d, want %d", got, want)
	}
	f.pawns = []entity.EntityID{2}
	if got := s.Gear.Wealth(); got != item.Meal.Value() {
		t.Fatalf("a pack off this map counted: %d", got)
	}
}
