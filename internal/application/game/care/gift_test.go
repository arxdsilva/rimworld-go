package care

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

const giftStack entity.EntityID = 70

// fakeGifts offers one dose: from a stack at (1, 1), or from the visitor's pack.
type fakeGifts struct {
	h      *fakeHost
	offer  bool
	pack   bool
	tol    int
	given  []entity.EntityID
	held   entity.EntityID
	isDose map[entity.EntityID]bool
	noTake bool
}

func newFakeGifts(h *fakeHost) *fakeGifts {
	return &fakeGifts{h: h, offer: true, isDose: map[entity.EntityID]bool{giftStack: true}}
}

func (f *fakeGifts) GiftFor(_, _ entity.EntityID, _ uint64) (item.Kind, entity.EntityID, int, int, bool) {
	if f.pack {
		return item.Beer, 0, 0, 0, f.offer
	}
	return item.Beer, giftStack, 1, 1, f.offer
}
func (f *fakeGifts) hold() (entity.EntityID, bool) {
	f.held = 71
	f.isDose[71] = true
	return 71, true
}
func (f *fakeGifts) Unpack(entity.EntityID, item.Kind) (entity.EntityID, bool) { return f.hold() }
func (f *fakeGifts) TakeOne(entity.EntityID, entity.EntityID) (entity.EntityID, bool) {
	if f.noTake {
		return 0, false
	}
	return f.hold()
}
func (f *fakeGifts) Tolerance(entity.EntityID, item.Kind) int { return f.tol }
func (f *fakeGifts) IsDose(s entity.EntityID) bool            { return f.isDose[s] }
func (f *fakeGifts) GiveDose(_, patient, stack entity.EntityID) {
	f.held = 0
	f.given = append(f.given, patient)
}

func giftScene() (*Care, *fakeHost, *fakeSocial, *fakeGifts) {
	_, h, s := visitScene()
	g := newFakeGifts(h)
	return New(h, Roles{Social: s, Gifts: g}), h, s, g
}

// giftTick is a tick the draw of the pair (visitor1, bedded) makes a gift of an
// untolerated dose.
func giftTick(t *testing.T, c *Care, h *fakeHost) uint64 {
	t.Helper()
	for now := uint64(0); now < 10000; now++ {
		if c.Giving.chosen(giftHash(now, visitor1, bedded), bedded, item.Beer) {
			return now
		}
	}
	t.Fatal("no tick in 10000 brings a gift")
	return 0
}

func TestGiftShareFollowsTolerance(t *testing.T) {
	t.Parallel()
	c, _, _, g := giftScene()
	count := func() int {
		n := 0
		for now := uint64(0); now < 10000; now++ {
			if c.Giving.chosen(giftHash(now, visitor1, bedded), bedded, item.Beer) {
				n++
			}
		}
		return n
	}
	if n := count(); n < 3700 || n > 4300 { // 4 to 6
		t.Fatalf("no tolerance: %d of 10000 are gifts", n)
	}
	g.tol = 50
	if n := count(); n < 2300 || n > 2700 { // 2 to 6
		t.Fatalf("half tolerance: %d of 10000 are gifts", n)
	}
	g.tol = 100
	if n := count(); n != 0 {
		t.Fatalf("full tolerance: %d of 10000 are gifts", n)
	}
}

func TestGiftDeterministic(t *testing.T) {
	t.Parallel()
	c, h, _, _ := giftScene()
	at := giftTick(t, c, h)
	for i := 0; i < 2; i++ {
		c, h, _, _ := giftScene()
		h.now = at
		if !c.Giving.Try(visitor1, bedded) {
			t.Fatalf("run %d: tick %d brings a gift", i, at)
		}
		if j := h.jobs[visitor1]; j.Kind != job.FeedPatient || j.Target != giftStack || j.Dest != bedded {
			t.Fatalf("job %+v", j)
		}
		if o, _ := h.Owner(bedded); o != visitor1 {
			t.Fatal("the patient is held for the gift")
		}
	}
	a, b := giftHash(at, visitor1, bedded), giftHash(at, bedded, visitor1)
	if a == b {
		t.Fatal("the pair's order is part of the draw")
	}
}

func TestGiftRefused(t *testing.T) {
	t.Parallel()
	for name, edit := range map[string]func(h *fakeHost, g *fakeGifts){
		"nothing allowed": func(_ *fakeHost, g *fakeGifts) { g.offer = false },
		"tolerated":       func(_ *fakeHost, g *fakeGifts) { g.tol = 100 },
		"already held":    func(h *fakeHost, _ *fakeGifts) { h.owners[bedded] = 9 },
		"cannot start":    func(h *fakeHost, _ *fakeGifts) { h.blocked[visitor1] = true },
	} {
		c, h, _, g := giftScene()
		h.now = giftTick(t, c, h)
		edit(h, g)
		if c.Giving.Try(visitor1, bedded) {
			t.Errorf("%s: stays a visit, got %+v", name, h.jobs[visitor1])
		}
	}
	if c := New(newFakeHost(), Roles{}); c.Giving.Try(visitor1, bedded) || c.Giving.IsGift(job.Record{Target: giftStack}) {
		t.Fatal("no role wired: no gifts")
	}
}

// TestGiftFromPack: once out of the pack the dose is on the job, whose end puts
// it back if the gift falls through.
func TestGiftFromPack(t *testing.T) {
	t.Parallel()
	c, h, _, g := giftScene()
	g.pack = true
	h.now = giftTick(t, c, h)
	if !c.Giving.Try(visitor1, bedded) {
		t.Fatal("from the pack")
	}
	if j := h.jobs[visitor1]; j.Carry != 71 || j.Target != 71 || j.TX != 9 || j.TY != 8 || j.Dest != bedded {
		t.Fatalf("job %+v: straight to the bedside with the dose", j)
	}
}

func TestGiftDo(t *testing.T) {
	t.Parallel()
	c, h, _, g := giftScene()
	j := job.Record{Kind: job.FeedPatient, Target: giftStack, Dest: bedded, State: job.Do}
	h.Place(visitor1, 9, 8) // beside the patient already
	c.Feed.Do(visitor1, j)
	got := h.jobs[visitor1]
	if len(g.given) != 1 || g.given[0] != bedded || got.State != job.Complete || got.Carry != 0 {
		t.Fatalf("given %v, job %+v", g.given, got)
	}
	// Patient no longer awake: the job ends and leaves the dose to the job's end.
	c, h, s, g := giftScene()
	s.asleep[bedded] = true
	c.Feed.Do(visitor1, job.Record{Kind: job.FeedPatient, Target: 71, Carry: 71, Dest: bedded, State: job.Do})
	if got := h.jobs[visitor1]; got.State != job.Complete || got.Carry != 71 || len(g.given) != 0 {
		t.Fatalf("job %+v, given %v", got, g.given)
	}
	// Stack gone on arrival.
	c, h, _, g = giftScene()
	g.noTake = true
	c.Feed.Do(visitor1, job.Record{Kind: job.FeedPatient, Target: giftStack, Dest: bedded, State: job.Do})
	if got := h.jobs[visitor1]; got.State != job.Fail || got.Fail != job.TargetGone {
		t.Fatalf("job %+v", got)
	}
}
