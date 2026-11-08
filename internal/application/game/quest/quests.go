package quest

import (
	"slices"

	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/faction"
)

// Error is a quests root error (a constant: the package holds no state).
type Error string

func (e Error) Error() string { return string(e) }

// ErrNoQuest: no quest has that id. ErrNoKind: the quest's kind was not
// passed to New. ErrOp: an unknown command.
const (
	ErrNoQuest Error = "quest: no such quest"
	ErrNoKind  Error = "quest: kind not registered"
	ErrOp      Error = "quest: unknown command"
	// ErrCannotOffer: that kind cannot be offered now (OpOffer).
	ErrCannotOffer Error = "quest: that kind cannot be offered now"
)

// Quests is the quests root: every quest ever offered, sorted by id (ids
// are issued in order, so appending keeps it sorted), the due heap of
// their timers and the cadence's next offer, the signal index and the
// hourly relation snapshot. The game keeps one, built with New.
type Quests struct {
	r     Roles
	kinds [qd.NumKinds]Kind
	defs  [qd.NumKinds]qd.Def
	list  []qd.Quest
	heap  dueHeap
	// due is each live quest's current timer; a heap entry that differs
	// is stale and skipped.
	due map[qd.ID]entry
	idx index
	// rel is the hourly per-faction relation snapshot (QST-012); empty
	// after New or Import, so the first Hourly checks every live quest.
	rel []faction.Relation
	// noGoodwill are the faction indexes whose goodwill the player
	// declines as a reward (QST-083), ascending.
	noGoodwill []int
	next       qd.ID
	cad        Cadence
	ver        uint64
	ids, fids  []qd.ID // scratch for routing and the relation check
}

// New builds the root on roles r with kinds, each indexed by its
// Def().Kind (a later kind of the same id wins; KindNone and unknown ids
// are ignored).
func New(r Roles, kinds []Kind) *Quests {
	q := &Quests{r: r, due: map[qd.ID]entry{}, idx: newIndex()}
	for _, k := range kinds {
		if k == nil {
			continue
		}
		d := k.Def()
		if d.Kind == qd.KindNone || d.Kind >= qd.NumKinds {
			continue
		}
		q.kinds[d.Kind], q.defs[d.Kind] = k, d
	}
	return q
}

// Version moves on every change the read model shows (the game's view
// memo, QST-154).
func (q *Quests) Version() uint64 { return q.ver }

// Get is quest id, a copy (ok false: no such quest).
func (q *Quests) Get(id qd.ID) (qd.Quest, bool) {
	i, ok := q.find(id)
	if !ok {
		return qd.Quest{}, false
	}
	return cloneQuest(q.list[i]), true
}

// Hourly runs every in-game hour: it pops only the due heap entries (an
// offer's accept window, an active quest's deadline, a part's timer, the
// cadence's offer), then diffs the per-faction relation snapshot and
// fails the quests of each changed faction (QST-012). Due entries go
// first, relation changes after (QST-160).
func (q *Quests) Hourly(now uint64) {
	q.popDue(now)
	q.relations(now)
}

// Daily runs at midnight: it schedules the cadence's first offer if none
// is pending (QST-020..024) and pops whatever is due.
func (q *Quests) Daily(now uint64) {
	if !q.cad.Started {
		q.scheduleOffer(0, now)
	}
	q.popDue(now)
}

// Signal routes s to the live quests it concerns (Signal's doc) in id
// order and applies each kind's verdict.
func (q *Quests) Signal(s Signal) {
	now := q.r.Clock.Now()
	q.ids = q.idx.route(q.ids, s)
	for _, id := range q.ids {
		q.signalOne(id, s, now)
	}
}

// Offer offers a quest of kind k now, outside the cadence (the long-range
// scanner's ore lump, QST-116; dev tools): when the kind may be offered
// and builds. It is the id issued.
func (q *Quests) Offer(k qd.KindID) (qd.ID, bool) { return q.OfferAt(k, -1) }

// OfferAt is Offer for world tile t (Ctx.Tile): the long-range scanner's
// ore site (QST-116).
func (q *Quests) OfferAt(k qd.KindID, t int) (qd.ID, bool) {
	if k >= qd.NumKinds || q.kinds[k] == nil {
		return 0, false
	}
	c := q.ctx(q.r.Clock.Now())
	c.Tile = t
	if !q.kinds[k].CanOffer(c) {
		return 0, false
	}
	return q.build(k, c)
}

// ctx is a kind's context at now.
func (q *Quests) ctx(now uint64) Ctx { return Ctx{R: q.r, Now: now, Tile: -1, q: q} }

// find is quest id's position in list (binary search: list is by id).
func (q *Quests) find(id qd.ID) (int, bool) {
	return slices.BinarySearchFunc(q.list, id, func(e qd.Quest, id qd.ID) int {
		switch {
		case e.ID < id:
			return -1
		case e.ID > id:
			return 1
		}
		return 0
	})
}

// kindOf is k's registered kind (nil: none).
func (q *Quests) kindOf(k qd.KindID) Kind {
	if k >= qd.NumKinds {
		return nil
	}
	return q.kinds[k]
}

// defOf is k's Def: the registered kind's, else the domain table's.
func (q *Quests) defOf(k qd.KindID) qd.Def {
	if q.kindOf(k) != nil {
		return q.defs[k]
	}
	return qd.DefOf(k)
}

// NoGoodwill is the faction refs (1 + index) whose goodwill the player
// declines as a reward, ascending (the preference dialog's read; nil-safe).
func (q *Quests) NoGoodwill() []int {
	if q == nil {
		return nil
	}
	out := make([]int, len(q.noGoodwill))
	for i, f := range q.noGoodwill {
		out[i] = f + 1
	}
	return out
}

// declines: the player turned goodwill rewards off for faction index f.
func (q *Quests) declines(f int) bool {
	_, found := slices.BinarySearch(q.noGoodwill, f)
	return found
}

// build makes quest of kind k at c.Now: fills the root's fields, lets the
// kind build it, starts it (qd.Offer), runs Accepted for an AutoAccept
// kind and posts the offer letter. A build that fails, or a giver or
// target already in the wrong relation, issues nothing.
func (q *Quests) build(k qd.KindID, c Ctx) (qd.ID, bool) {
	kind, d := q.kinds[k], q.defs[k]
	n := qd.Quest{ID: q.next + 1, Kind: k, Chosen: -1, Seed: c.Seed(),
		Stars: qd.RollStars(c.Intn(100)), Points: q.r.World.Points()}
	if !kind.Build(c, &n) {
		return 0, false
	}
	n.ID, n.Kind = q.next+1, k
	if _, bad := qd.FailOnRelation(n, d, relOf(q.r.Goodwill, n.Giver), relOf(q.r.Goodwill, n.Target)); bad {
		return 0, false
	}
	n = qd.Offer(n, c.Now, d)
	q.next = n.ID
	q.list = append(q.list, n)
	i := len(q.list) - 1
	if q.list[i].State == qd.Active {
		kind.Accepted(c, &q.list[i])
		if d.RewardAt == qd.OnAccept {
			pay(q.r, d, &q.list[i], 0)
		}
	}
	q.touched(i)
	post(q.r.Letters, LetterOffer, q.list[i])
	return n.ID, true
}

// relOf is faction ref's (1 + index) relation now; Neutral for none.
func relOf(g Goodwill, ref int) faction.Relation {
	if ref <= 0 {
		return faction.Neutral
	}
	return g.Relation(ref - 1)
}

// touched re-indexes and re-schedules quest i after any change and moves
// the version.
func (q *Quests) touched(i int) {
	q.idx.set(q.list[i])
	q.schedule(q.list[i])
	q.ver++
}

// schedule pushes quest n's next timer (qd.Due) unless it is already the
// one pending; a terminal quest has none.
func (q *Quests) schedule(n qd.Quest) {
	if n.State.Terminal() {
		delete(q.due, n.ID)
		return
	}
	t, ev, part, ok := qd.Due(n)
	if !ok {
		delete(q.due, n.ID)
		return
	}
	e := entry{Tick: t, ID: n.ID, Ev: ev, Part: part}
	if old, has := q.due[n.ID]; has && old == e {
		return
	}
	q.due[n.ID] = e
	q.heap.push(e)
}
