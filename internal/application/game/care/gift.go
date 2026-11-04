package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// Gifts for the sick (CSE-007). A visit to a friend lying awake in bed may
// become bringing it a drug its own policy allows, from the visitor's pack
// first, else the nearest stack. The choice is a hash of the tick and the pair,
// weighted drug giftWeight to visit visitWeight (ours), the drug's share falling
// with the patient's tolerance to it. Cooldown: the visit's own slot and the
// policy's dose gap.
const (
	giftWeight  = 4
	visitWeight = 6
)

// Gifts is what bringing a dose needs from the simulation.
type Gifts interface {
	// GiftFor is the dose visitor may bring patient: kind k, from its pack (stack
	// 0) or from the nearest stack at (x, y). False: the rule is off or the
	// patient's policy allows none now.
	GiftFor(visitor, patient entity.EntityID, turn uint64) (k item.Kind, stack entity.EntityID, x, y int, ok bool)
	// Unpack brings one k out of id's pack into its hands.
	Unpack(id entity.EntityID, k item.Kind) (entity.EntityID, bool)
	// TakeOne has id, at stack, pick up one dose in its hands.
	TakeOne(id, stack entity.EntityID) (entity.EntityID, bool)
	// Tolerance is patient's tolerance to k, in percent.
	Tolerance(patient entity.EntityID, k item.Kind) int
	// IsDose reports stack is a stack of a drug.
	IsDose(stack entity.EntityID) bool
	// GiveDose has patient take the dose feeder holds as stack.
	GiveDose(feeder, patient, stack entity.EntityID)
}

// Giving is the gifts component.
type Giving struct {
	c *Care
	r Gifts
}

// hash is the pair's draw at tick now (no random numbers: visitSlot's way).
func giftHash(now uint64, v, p entity.EntityID) uint64 {
	return randx.SplitMix64(now ^ randx.SplitMix64(uint64(v)<<32|uint64(p)))
}

// chosen: the draw makes this visit a gift of k.
func (g *Giving) chosen(draw uint64, patient entity.EntityID, k item.Kind) bool {
	gift := giftWeight * max(100-g.r.Tolerance(patient, k), 0)
	return draw>>32%uint64(gift+visitWeight*100) < uint64(gift)
}

// Try makes visitor's visit to patient a gift, when the draw says so and there
// is something to bring; false: the visit goes on as a visit.
func (g *Giving) Try(visitor, patient entity.EntityID) bool {
	h := g.c.h
	if g.r == nil {
		return false
	}
	if _, held := h.Owner(patient); held {
		return false // someone is seeing to it already
	}
	draw := giftHash(h.Now(), visitor, patient)
	k, stack, x, y, ok := g.r.GiftFor(visitor, patient, draw)
	if !ok || !g.chosen(draw, patient, k) {
		return false
	}
	rec := job.Record{Kind: job.FeedPatient, Target: stack, TX: x, TY: y, Dest: patient}
	if stack != 0 {
		if !h.StartJob(visitor, rec) {
			return false
		}
		h.Claim(visitor, patient)
		return true
	}
	return g.fromPack(visitor, patient, k, rec)
}

// fromPack starts a gift from visitor's pack: straight to the patient's bedside
// with the dose in hand. Once the dose is out the job carries it even if it
// could not start, so the job's end puts it back.
func (g *Giving) fromPack(visitor, patient entity.EntityID, k item.Kind, rec job.Record) bool {
	h := g.c.h
	px, py, _ := h.Pos(patient)
	sx, sy, ok := h.BesideSpot(visitor, px, py)
	if !ok {
		return false
	}
	carry, ok := g.r.Unpack(visitor, k)
	if !ok {
		return false
	}
	rec.Target, rec.Carry, rec.TX, rec.TY = carry, carry, sx, sy
	if h.StartJob(visitor, rec) {
		h.Claim(visitor, patient)
	}
	return true
}

// IsGift: feeding job j brings a dose, not a meal.
func (g *Giving) IsGift(j job.Record) bool { return g.r != nil && g.r.IsDose(j.Target) }

// Do runs one tick of a gift (Feeding.Do hands it over): at the stack it picks
// up a dose and walks to the patient; beside it, the dose is given. A patient no
// longer lying awake ends the job, and what the visitor holds falls with it.
func (g *Giving) Do(id entity.EntityID, j job.Record) {
	c, h, p := g.c, g.c.h, j.Dest
	if !c.Visits.wantsCompany(p) {
		j.State = job.Complete
		h.SetJob(id, j)
		return
	}
	if j.Carry == 0 {
		carry, ok := g.r.TakeOne(id, j.Target)
		if !ok {
			j.State, j.Fail = job.Fail, job.TargetGone
			h.SetJob(id, j)
			return
		}
		j.Carry, j.Target = carry, carry
		var there bool
		if j, there = c.Feed.WalkBeside(id, p, j); !there {
			return
		}
	}
	if !c.adjacent(id, p) {
		c.Feed.Chase(id, p, j, feedChase)
		return
	}
	stack := j.Carry
	j.Carry, j.State = 0, job.Complete
	h.SetJob(id, j)
	g.r.GiveDose(id, p, stack)
}
