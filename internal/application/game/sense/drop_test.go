package sense

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// orders is the Gear tab's side of the errand game (GearOrders).
type orders struct {
	prisoner, fallen, noDose, broken bool
	outfit                           craft.Outfit
}

func (o *orders) Breaking(entity.EntityID) bool { return o.broken }

func (o *orders) Up(entity.EntityID) bool                     { return !o.fallen }
func (o *orders) Prisoner(entity.EntityID) bool               { return o.prisoner }
func (o *orders) Outfit(entity.EntityID) craft.Outfit         { return o.outfit }
func (o *orders) SetOutfit(_ entity.EntityID, w craft.Outfit) { o.outfit = w }
func (o *orders) MayDose(entity.EntityID, item.Kind) bool     { return !o.noDose }

// tabScene is colonist 1 with steel, a meal and a beer in its pack, hungry
// enough to eat, wearing slots 3 and 5, armed and with a spare.
func tabScene(t *testing.T) (*Sense, *errand, map[item.Kind]int) {
	t.Helper()
	f := &fakeGear{pawns: []entity.EntityID{1}, manip: 1000, stacks: map[entity.EntityID]item.Stack{
		10: {Kind: item.Steel, Qty: 20}, 11: {Kind: item.Meal, Qty: 2}, 12: {Kind: item.Beer, Qty: 3},
	}}
	s, e := errandScene(f)
	ids := map[item.Kind]int{}
	for stack, k := range map[entity.EntityID]item.Kind{10: item.Steel, 11: item.Meal, 12: item.Beer} {
		if !s.Gear.Stow(1, stack, f.stacks[stack].Qty) {
			t.Fatalf("stow %s", k)
		}
	}
	for _, en := range s.Gear.Pack(1).Items {
		ids[en.Stack.Kind] = int(en.ID)
	}
	e.hunger, e.worn, e.armed, e.spare = needs.EatAt, []int{3, 5}, true, true
	return s, e, ids
}

func drop(s *Sense, pawn uint64, mode uint8, index int) {
	s.Apply(command.Command{Kind: command.DropGear, Pawn: pawn, Mode: mode, Index: index})
}

// idOf is the entry of kind k in 1's pack now (an entry taken whole and put
// back has a new ID).
func idOf(s *Sense, k item.Kind) uint32 {
	id, _ := s.Gear.Pack(1).Find(k)
	return id
}

func packQty(s *Sense, k item.Kind) int {
	n := 0
	for _, en := range s.Gear.Pack(1).Items {
		if en.Stack.Kind == k {
			n += en.Stack.Qty
		}
	}
	return n
}

func TestDropPackEntryFallsAtOnce(t *testing.T) {
	t.Parallel()
	s, e, ids := tabScene(t)
	drop(s, 1, modePack, ids[item.Steel])
	if packQty(s, item.Steel) != 0 || len(e.down) != 1 || e.down[0].Kind != item.Steel || e.down[0].Qty != 20 {
		t.Fatalf("the whole entry should be at its feet: pack %+v, down %+v", s.Gear.Pack(1), e.down)
	}
	if len(e.starts) != 0 || packQty(s, item.Meal) != 2 {
		t.Fatalf("a pack drop takes no job and nothing else: %d jobs", len(e.starts))
	}
}

func TestDropWeaponAndSpareAtOnce(t *testing.T) {
	t.Parallel()
	s, e, _ := tabScene(t)
	drop(s, 1, modeWeapon, 0)
	if e.armed {
		t.Fatal("the weapon was not dropped")
	}
	drop(s, 1, modeSpare, 0)
	if e.spare || len(e.starts) != 0 {
		t.Fatalf("the spare should drop at once: spare %v, %d jobs", e.spare, len(e.starts))
	}
}

func TestDropGarmentTakesOffFirst(t *testing.T) {
	t.Parallel()
	s, e, _ := tabScene(t)
	e.hunger = needs.Full // no spare-meal errand in the way
	drop(s, 1, modeApparel, 1)
	j := e.jobs[1]
	if len(e.starts) != 1 || j.Kind != job.UnloadPack || !j.Forced || j.BillIndex != 5 || j.WorkLeft != dropWearWait {
		t.Fatalf("a garment comes off through a forced job: %+v", j)
	}
	if len(e.takenOff) != 0 {
		t.Fatal("the garment came off before its time")
	}
	if !e.until(dropWearWait+5, func() bool { return len(e.takenOff) == 1 }) || e.takenOff[0] != 5 {
		t.Fatalf("slot 5 should come off: %v", e.takenOff)
	}
}

func TestPrisonerGarmentDropsAtOnce(t *testing.T) {
	t.Parallel()
	s, e, _ := tabScene(t)
	e.pawns, e.prisoner = nil, true
	delete(s.Packs, 1) // no prisoner keeps a pack
	drop(s, 1, modeApparel, 0)
	if len(e.takenOff) != 1 || e.takenOff[0] != 3 || len(e.starts) != 0 {
		t.Fatalf("a prisoner's garment comes off at once, no job: %v, %d jobs", e.takenOff, len(e.starts))
	}
	if why := s.Gear.DropWhy(1, ModeAny, 0); why != "" {
		t.Fatalf("a prisoner's gear may be dropped: %q", why)
	}
	if why := s.Gear.EatWhy(1, 1); why != WhyCaptive {
		t.Fatalf("a prisoner takes no Eat order: %q", why)
	}
}

func TestDropRefusals(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name        string
		set         func(e *errand)
		pawn        entity.EntityID
		mode, index int
		want        string
	}{
		{"guest or hostile", nil, 7, modeWeapon, 0, WhyNotOurs},
		{"downed", func(e *errand) { e.fallen = true }, 1, modeWeapon, 0, WhyDown},
		{"downed, the pawn alone", func(e *errand) { e.fallen = true }, 1, ModeAny, 0, WhyDown},
		{"mental break", func(e *errand) { e.broken = true }, 1, modeWeapon, 0, WhyBreak},
		{"mental break, the pawn alone", func(e *errand) { e.broken = true }, 1, ModeAny, 0, WhyBreak},
		{"no such entry", nil, 1, modePack, 99, WhyGone},
		{"negative entry", nil, 1, modePack, -1, WhyGone},
		{"negative garment", nil, 1, modeApparel, -1, WhyGone},
		{"no such garment", nil, 1, modeApparel, 2, WhyGone},
		{"unarmed", func(e *errand) { e.armed = false }, 1, modeWeapon, 0, WhyGone},
		{"no spare", func(e *errand) { e.spare = false }, 1, modeSpare, 0, WhyGone},
		{"unknown mode", nil, 1, 4, 0, WhyGone},
	} {
		s, e, _ := tabScene(t)
		if c.set != nil {
			c.set(e)
		}
		armed, spare, before := e.armed, e.spare, len(s.Gear.Pack(1).Items)
		if got := s.Gear.DropWhy(c.pawn, c.mode, c.index); got != c.want {
			t.Fatalf("%s: DropWhy %q, want %q", c.name, got, c.want)
		}
		if got := s.Gear.Drop(c.pawn, c.mode, c.index); got != c.want {
			t.Fatalf("%s: Drop %q, want %q", c.name, got, c.want)
		}
		if e.armed != armed || e.spare != spare || len(s.Gear.Pack(1).Items) != before || len(e.down)+len(e.takenOff)+len(e.starts) != 0 {
			t.Fatalf("%s: a refused drop changed something", c.name)
		}
	}
	s, _, _ := tabScene(t)
	if why := s.Gear.DropWhy(1, ModeAny, 0); why != "" {
		t.Fatalf("a standing colonist may drop: %q", why)
	}
}

func TestConsumeFoodFromPack(t *testing.T) {
	t.Parallel()
	s, e, ids := tabScene(t)
	e.jobs[1] = job.Record{Kind: job.Haul, State: job.Do}
	s.Apply(command.Command{Kind: command.ConsumeFromPack, Pawn: 1, Index: ids[item.Meal]})
	j := e.jobs[1]
	if j.Kind != job.Eat || !j.Forced || j.Count != 2 || j.Carry == 0 || j.Target != j.Carry || j.WorkLeft != needs.EatWorkTicks {
		t.Fatalf("the meal is eaten in place, as a forced job: %+v", j)
	}
	if st := e.stacks[j.Carry]; st.Kind != item.Meal || st.Qty != 2 || st.Carrier != 1 || packQty(s, item.Meal) != 0 {
		t.Fatalf("the entry should be in hand: %+v", st)
	}
	e.Interrupt(1) // stopped before eating: it goes back
	if packQty(s, item.Meal) != 2 {
		t.Fatalf("the uneaten meal should be back in the pack: %d", packQty(s, item.Meal))
	}
	e.hunger = tabFullAt
	if why := s.Gear.Consume(1, idOf(s, item.Meal)); why != WhyFull || e.jobs[1].Kind != job.None {
		t.Fatalf("a full colonist is refused: %q", why)
	}
	e.hunger = tabFullAt - 1
	if why := s.Gear.EatWhy(1, idOf(s, item.Meal)); why != "" {
		t.Fatalf("just below the mark it may eat: %q", why)
	}
	if why := s.Gear.Consume(1, uint32(ids[item.Steel])); why != WhyInedible || packQty(s, item.Steel) != 20 {
		t.Fatalf("steel is not eaten: %q", why)
	}
	if why := s.Gear.Consume(1, 99); why != WhyGone {
		t.Fatalf("no such entry: %q", why)
	}
}

func TestConsumeDrugFromPack(t *testing.T) {
	t.Parallel()
	s, e, ids := tabScene(t)
	e.hunger = needs.Full // a drug needs no hunger
	s.Apply(command.Command{Kind: command.ConsumeFromPack, Pawn: 1, Index: ids[item.Beer]})
	j := e.jobs[1]
	if j.Kind != job.Ingest || !j.Forced || j.Carry == 0 || j.Target != j.Carry || j.WorkLeft != doseTicks {
		t.Fatalf("one dose is taken through the dose job: %+v", j)
	}
	if st := e.stacks[j.Carry]; st.Qty != 1 || packQty(s, item.Beer) != 2 {
		t.Fatalf("one dose in hand, two left: %+v, %d", st, packQty(s, item.Beer))
	}
	e.Interrupt(1)
	e.noDose = true
	if why := s.Gear.Consume(1, idOf(s, item.Beer)); why != WhyPolicy || packQty(s, item.Beer) != 3 {
		t.Fatalf("the drug policy refuses it: %q", why)
	}
	e.noDose, e.pawns = false, nil
	if why := s.Gear.EatWhy(1, idOf(s, item.Beer)); why != WhyNotOurs {
		t.Fatalf("not a colonist: %q", why)
	}
	e.pawns, e.fallen = []entity.EntityID{1}, true
	if why := s.Gear.EatWhy(1, idOf(s, item.Beer)); why != WhyDown {
		t.Fatalf("downed: %q", why)
	}
}

func TestConsumeThatCannotStartGoesBack(t *testing.T) {
	t.Parallel()
	s, e, ids := tabScene(t)
	e.startOK = false
	if why := s.Gear.Consume(1, uint32(ids[item.Meal])); why != WhyBusy {
		t.Fatalf("a job that can't start is refused: %q", why)
	}
	if packQty(s, item.Meal) != 2 || e.jobs[1].Carry != 0 {
		t.Fatalf("the meal should be back in the pack: %d", packQty(s, item.Meal))
	}
}

func TestLockGarment(t *testing.T) {
	t.Parallel()
	s, e, _ := tabScene(t)
	lock := func(pawn uint64, n int, on bool) {
		s.Apply(command.Command{Kind: command.SetApparelForced, Pawn: pawn, Index: n, Enabled: on})
	}
	lock(1, 1, true)
	if !e.outfit.IsForced(5) || e.outfit.IsForced(3) {
		t.Fatalf("slot 5 should be forced: %08b", e.outfit.Forced)
	}
	lock(1, 1, false)
	if e.outfit.IsForced(5) {
		t.Fatal("the lock did not come off")
	}
	lock(7, 0, true)
	lock(1, 2, true)
	lock(1, -1, true)
	if e.outfit.Forced != 0 {
		t.Fatalf("a refused lock changed the outfit: %08b", e.outfit.Forced)
	}
	if why := s.Gear.Lock(1, 4, true); why != WhyNoGarment {
		t.Fatalf("no such garment: %q", why)
	}
}

func TestPackGearFacts(t *testing.T) {
	t.Parallel()
	s, e, _ := tabScene(t)
	got := map[item.Kind]gearview.Pack{}
	for _, en := range s.Gear.Pack(1).Items {
		got[en.Stack.Kind] = s.Gear.PackGear(1, en)
	}
	want := map[item.Kind]gearview.Pack{
		item.Steel: {Grams: item.Steel.MassGrams() * 20, Use: gearview.UseNone, Why: WhyInedible},
		item.Meal:  {Grams: item.Meal.MassGrams() * 2, Use: gearview.UseEat},
		item.Beer:  {Grams: item.Beer.MassGrams() * 3, Use: gearview.UseDrug},
	}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("%s: %+v, want %+v", k, got[k], w)
		}
	}
	e.hunger = needs.Full
	for _, en := range s.Gear.Pack(1).Items {
		if en.Stack.Kind == item.Meal && s.Gear.PackGear(1, en).Why != WhyFull {
			t.Fatal("a full colonist's meal row should say why")
		}
	}
}
