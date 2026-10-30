package game

import (
	"slices"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Clothes. Each colonist wears at most one garment per slot (body, head)
// over its everyday clothes; garments shift its comfortable temperature
// band. A colonist dresses in the best outfit it can get for the day
// (PLAY-017): before work at every job pick, and in the middle of plain
// work when a better one turns up. Pieces it should not wear come off at
// once; then it walks to each new piece in turn. A slot that just changed
// stays as it is for dressCool (unless the colonist is freezing or
// sweltering), and a slot the player dressed ("Wear X") stays the
// player's until another garment is ordered for it.

const (
	// dressGain: a change must spare at least 1 C of discomfort.
	dressGain = 10
	// dressCool is how long a slot that changed stays as it is.
	dressCool = 2 * clock.TicksPerHour
	// dressEvery: a colonist at work looks at its clothes once per this
	// many ticks (staggered by id).
	dressEvery = 60
	// dressTries caps the garments one check sets off for.
	dressTries = 3
)

// worn is what a colonist has on, piece by piece with each garment's
// material, quality and wear, and the slots the player chose
// (craft/garment.go, PRD-2).
type worn = craft.Outfit

// wearSlots are the slots garments go in.
var wearSlots = [...]item.Slot{item.Body, item.Head, item.Legs, item.Belt} // DEF-2: flak pants; CBT-3: belts (only by order)

// bandsOf is a colonist's comfortable range given what it wears.
func (g *Game) bandsOf(id entity.EntityID) exposure.Bands {
	c, h := g.comfortShift(id)
	return shifted(g.apparel[id].Bands(), c, h) // craft/garment.go
}

// shifted is b moved by a colonist's comfort shift: cold is added to the
// floor, heat to the ceiling (skills.go: build and traits, Cold-hardy).
func shifted(b exposure.Bands, cold, heat int) exposure.Bands {
	if cold != 0 || heat != 0 {
		b = b.Wearing(-cold, heat)
	}
	return b
}

// wear puts on the garment stack sid (one item), dropping whatever was in
// that slot; forced: the player ordered it, and the slot becomes theirs.
func (g *Game) wear(id, sid entity.EntityID, st item.Stack, forced bool) {
	pos, _ := g.positions.Get(id)
	w := g.apparel[id]
	slot := st.Kind.Slot()
	if old := w.On[slot]; old.Kind != 0 {
		g.dropStackOf(old.Stack(pos.X, pos.Y), pos.X, pos.Y)
	}
	w.On[slot] = craft.PieceOf(st)
	w.SetForced(slot, forced)
	g.apparel[id] = w
	if st.Qty <= 1 {
		g.deleteItem(sid)
	} else {
		st.Qty--
		g.putStack(sid, st)
	}
	g.dressChanged(id, slot)
	g.dressNote(id, logPutOn, st.Kind)
}

// dressNote records a change of clothes in colonist id's own log (the
// inspector's Log tab), not the message feed: a whole colony dressing at
// landfall or at a change of season would bury the real news.
func (g *Game) dressNote(id entity.EntityID, kind uint8, k item.Kind) {
	g.logEvent(id, logEntry{Kind: kind, Name: k.String()})
}

// Wearing lists the garments a colonist has on (dev and tests).
func (g *Game) Wearing(id entity.EntityID) []string {
	var out []string
	w := g.apparel[id]
	for _, s := range wearSlots {
		if k := w.Kind(s); k != 0 {
			out = append(out, k.String())
		}
	}
	return out
}

// wornForced is, per garment in Wearing's order, whether the player chose
// it (PawnView.ApparelForced).
func (g *Game) wornForced(id entity.EntityID) []bool {
	var out []bool
	w := g.apparel[id]
	for _, s := range wearSlots {
		if w.Kind(s) != 0 {
			out = append(out, w.IsForced(s))
		}
	}
	return out
}

// changingInto names the garment colonist id is walking to put on ("":
// none; PawnView.ChangingInto).
func (g *Game) changingInto(id entity.EntityID) string {
	j, _ := g.jobs.Get(id)
	if j.Kind != job.Equip || !j.Active() {
		return ""
	}
	if sp := g.stacks.Ref(j.Target); sp != nil && sp.Kind.IsApparel() {
		return sp.Kind.String()
	}
	return ""
}

// wearOpt is one choice for a slot: kind (0: bare) from stack sid at
// (x, y), d away; sid 0 is what the colonist has on now, or bare.
type wearOpt struct {
	kind    item.Kind
	sid     entity.EntityID
	x, y, d int
}

// outfit is one choice per slot and how it suits the day.
type outfit struct {
	pick                [craft.Slots]wearOpt
	miss, surplus, walk int
}

// worn is the set o puts on.
func (o *outfit) worn() worn {
	var w worn
	for _, s := range wearSlots {
		w.On[s].Kind = o.pick[s].kind
	}
	return w
}

// next is the nearest garment o still has to fetch (sid 0: none).
func (o *outfit) next() wearOpt {
	var n wearOpt
	for _, s := range wearSlots {
		if p := o.pick[s]; p.sid != 0 && (n.sid == 0 || p.d < n.d) {
			n = p
		}
	}
	return n
}

// better orders outfits: less discomfort, then less surplus insulation,
// then a shorter walk.
func (o *outfit) better(b *outfit) bool {
	if o.miss != b.miss {
		return o.miss < b.miss
	}
	if o.surplus != b.surplus {
		return o.surplus < b.surplus
	}
	return o.walk < b.walk
}

// maxWearOpts sizes the per-slot option buffers (keep, bare and one per
// apparel kind; more only allocate).
const maxWearOpts = 8

// dressPlan is colonist id's best outfit for the day, and whether it is
// worth changing into: at least dressGain less discomfort than what it has
// on, judged with its comfort shift on both sides. Slots the player chose,
// and slots in their cooldown, keep what they have. With full false the
// plan is optimistic: every kind in the apparel index counts as lying at
// its feet. Garments only ever drop out of the full plan's choices, so
// when the optimistic plan finds nothing worth the change neither does the
// full one: the cheap gate busy colonists are checked with.
func (g *Game) dressPlan(id entity.EntityID, full bool) (outfit, bool) {
	cur := g.apparel[id]
	locked := g.dressLocked(id, cur)
	var buf [craft.Slots][maxWearOpts]wearOpt
	var opts [craft.Slots][]wearOpt
	for _, s := range wearSlots {
		opts[s] = append(buf[s][:0], wearOpt{kind: cur.Kind(s)}) // keep it on
		if cur.Kind(s) != 0 && !locked[s] {
			opts[s] = append(opts[s], wearOpt{}) // take it off
		}
	}
	if full {
		g.wearCandidates(id, cur, locked, &opts)
	} else {
		for _, k := range g.apparelKinds() {
			if s := k.Slot(); !locked[s] && k != cur.Kind(s) {
				opts[s] = append(opts[s], wearOpt{kind: k})
			}
		}
	}
	mean := int(g.climate.DailyMean(g.clock.Now()))
	c, h := g.comfortShift(id)
	var best outfit
	for i, b := range opts[item.Body] {
		for j, hd := range opts[item.Head] {
			for k, lg := range opts[item.Legs] {
				var o outfit
				o.pick[item.Body], o.pick[item.Head], o.pick[item.Legs], o.pick[item.Belt] = b, hd, lg, opts[item.Belt][0]
				bands := shifted(o.worn().Bands(), c, h) // craft/garment.go: fit and surplus too
				o.miss, o.surplus, o.walk = craft.DayFit(bands, mean), craft.DaySurplus(bands, mean), b.d+hd.d+lg.d
				if i == 0 && j == 0 && k == 0 || o.better(&best) {
					best = o
				}
			}
		}
	}
	return best, craft.DayFit(shifted(cur.Bands(), c, h), mean)-best.miss >= dressGain
}

// wearCandidates adds to opts, per slot, the nearest garment of each kind
// colonist id could go and put on: lying free (not carried, forbidden,
// reserved by another or put off), inside its allowed area and reachable.
// Kinds it already wears and locked slots are skipped.
func (g *Game) wearCandidates(id entity.EntityID, cur worn, locked [craft.Slots]bool, opts *[craft.Slots][]wearOpt) {
	pos, _ := g.positions.Get(id)
	for _, sid := range g.apparelStock(true) {
		sp := g.stacks.Ref(sid)
		if sp == nil || sp.Qty <= 0 || sp.Carrier != 0 || sp.Forbidden || sp.Tainted() { // PRD-2: a dead man's clothes only by order
			continue
		}
		s := sp.Kind.Slot()
		if locked[s] || sp.Kind == cur.Kind(s) {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, sp.X, sp.Y)
		i := slices.IndexFunc(opts[s], func(o wearOpt) bool { return o.sid != 0 && o.kind == sp.Kind })
		if i >= 0 && opts[s][i].d <= d {
			continue // a nearer one of this kind (ids ascend: ties keep the first)
		}
		if owner, taken := g.reserve.EntityOwner(sid); taken && owner != id {
			continue
		}
		if g.postponedFor(id, job.Equip, sid, 0, 0) || !g.areaAllows(id, sp.X, sp.Y) || !g.reachable(id, sp.X, sp.Y) {
			continue
		}
		o := wearOpt{kind: sp.Kind, sid: sid, x: sp.X, y: sp.Y, d: d}
		if i >= 0 {
			opts[s][i] = o
		} else {
			opts[s] = append(opts[s], o)
		}
	}
}

// dressLocked reports the slots colonist id keeps as they are: the
// player's choices, and slots in their cooldown unless it is freezing or
// sweltering where it stands.
func (g *Game) dressLocked(id entity.EntityID, cur worn) [craft.Slots]bool {
	var locked [craft.Slots]bool
	urgent, known := false, false
	for _, s := range wearSlots {
		switch {
		case cur.IsForced(s):
			locked[s] = true
		case g.dressCooling(id, s):
			if !known {
				urgent, known = g.dressUrgent(id), true
			}
			locked[s] = !urgent
		}
	}
	return locked
}

// dressUrgent reports a colonist with a Freezing or Sweltering thought
// (rooms.go thresholds).
func (g *Game) dressUrgent(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	t, b := int(g.temperatureAt(pos.X, pos.Y)), g.bandsOf(id)
	return t < b.Freezing() || t > b.Sweltering()
}

// dressCoolKey is the postponed entry (saved) holding slot s of colonist id
// after a change: an Equip with no target, at (-1, slot).
func dressCoolKey(id entity.EntityID, s item.Slot) postKey {
	return postKey{Pawn: id, Kind: job.Equip, X: -1, Y: int(s)}
}

// dressCooling reports slot s of colonist id in its cooldown.
func (g *Game) dressCooling(id entity.EntityID, s item.Slot) bool {
	k := dressCoolKey(id, s)
	return g.postponedFor(id, k.Kind, k.Target, k.X, k.Y)
}

// dressChanged starts slot s's cooldown after a change.
func (g *Game) dressChanged(id entity.EntityID, s item.Slot) {
	g.putOff(dressCoolKey(id, s), uint64(g.clock.Now())+dressCool)
}

// undress takes off at once what colonist id has on and outfit o leaves
// bare, dropping it at its feet.
func (g *Game) undress(id entity.EntityID, o *outfit) {
	for _, s := range wearSlots {
		w := g.apparel[id]
		p := w.On[s]
		k := p.Kind
		if k == 0 || o.pick[s].kind != 0 {
			continue
		}
		pos, _ := g.positions.Get(id)
		w.On[s] = craft.Piece{}
		w.SetForced(s, false)
		g.apparel[id] = w
		g.dropStackOf(p.Stack(pos.X, pos.Y), pos.X, pos.Y)
		g.dressChanged(id, s)
		g.dressNote(id, logTookOff, k)
	}
}

// dress changes colonist id into its best outfit: unwanted pieces come off
// at once, then it sets off for the nearest new piece (a later check takes
// the next). A fetch that fails out of reach or out of area was put off by
// startJob, so the next best is tried. True when it set off.
func (g *Game) dress(id entity.EntityID) bool {
	for range dressTries {
		o, ok := g.dressPlan(id, true)
		if !ok {
			return false
		}
		g.undress(id, &o)
		n := o.next()
		if n.sid == 0 {
			return false
		}
		if g.startJob(id, job.Record{Kind: job.Equip, Target: n.sid, TX: n.x, TY: n.y}) {
			return true
		}
		if j, _ := g.jobs.Get(id); j.Fail != job.Unreachable {
			return false // reserved, or the path search deferred: next tick
		}
	}
	return false
}

// assignWear is the dress check of free time (tick.go: non-work blocks,
// and when no work was found).
func (g *Game) assignWear(id entity.EntityID) bool { return g.dressCheck(id) }

// dressCheck changes colonist id into a better outfit before it looks for
// work (PLAY-017); true when it set off for a garment.
func (g *Game) dressCheck(id entity.EntityID) bool {
	if _, ok := g.dressPlan(id, false); !ok {
		return false
	}
	return g.dress(id)
}

// dressBreaks reports work j that may be dropped for clothes: plain work,
// not a need, an order, or a load being carried.
func dressBreaks(j job.Record) bool {
	if j.Forced || j.Carry != 0 {
		return false
	}
	switch j.Kind {
	case job.Haul, job.Construct, job.Deconstruct, job.Harvest, job.Cut, job.Mine, job.Smooth, job.Sow,
		job.Bill, job.Refuel, job.Repair, job.Research, job.Clean, job.Bury, job.Roof, job.Unroof,
		job.Milk, job.Shear, job.ClearSnow:
		return true
	}
	return false
}

// dressInterrupt drops colonist id's interruptible work job j for a better
// outfit (PLAY-017), once per dressEvery ticks. Pieces to take off come
// off without stopping the work; a piece to fetch abandons it the way
// drafting does (tick.go interrupt).
func (g *Game) dressInterrupt(id entity.EntityID, j job.Record) {
	if (uint64(g.clock.Now())+uint64(id))%dressEvery != 0 || !dressBreaks(j) || g.drafted(id) {
		return
	}
	if _, ok := g.dressPlan(id, false); !ok {
		return
	}
	o, ok := g.dressPlan(id, true)
	if !ok {
		return
	}
	if o.next().sid == 0 {
		g.undress(id, &o)
		return
	}
	g.interrupt(id, job.Interrupted)
	g.dress(id)
}

// apparelIndex is every apparel stack on the map (free, carried or
// forbidden), ids ascending; never saved. noteApparel adds spawned stacks;
// a read adds stacks split off without SpawnStack (item ids are handed out
// ascending and appended in order, so new ones are those above mark),
// drops gone ones when asked, and rebuilds the lot once an in-game hour.
// gen (apparelGen) bumps whenever stacks join; kinds, the distinct kinds,
// is taken per gen for the optimistic plan (gone stacks may linger in it
// until the next validating read: it only ever over-counts).
type apparelIndex struct {
	ids      []entity.EntityID
	mark     entity.EntityID
	hour     uint64 // 1 + the hour of the last rebuild (0: never built)
	gen      uint64
	kindsGen uint64 // 1 + the gen kinds was taken at
	kinds    []item.Kind
}

// apparelIdx is the game's apparel index.
func (g *Game) apparelIdx() *apparelIndex { return &g.clothes }

// noteApparel adds new apparel stack id to the free-clothes index
// (PLAY-017).
func (g *Game) noteApparel(id entity.EntityID) {
	x := g.apparelIdx()
	if x.add(id) {
		x.gen++
	}
}

// add puts id in the index; false when it was there.
func (x *apparelIndex) add(id entity.EntityID) bool {
	i, found := slices.BinarySearch(x.ids, id)
	if found {
		return false
	}
	x.ids = slices.Insert(x.ids, i, id)
	return true
}

// apparelStock is the apparel index brought up to date (validate: gone
// stacks dropped too). The slice is the index's own: read only.
func (g *Game) apparelStock(validate bool) []entity.EntityID {
	x := g.apparelIdx()
	if hour := uint64(g.clock.Now())/clock.TicksPerHour + 1; x.hour != hour {
		x.hour, x.ids, x.mark = hour, x.ids[:0], 0
		x.gen++
	}
	n := len(g.itemIDs)
	i := n
	for i > 0 && g.itemIDs[i-1] > x.mark {
		i--
	}
	for _, sid := range g.itemIDs[i:] {
		if sp := g.stacks.Ref(sid); sp != nil && sp.Kind.IsApparel() && x.add(sid) {
			x.gen++
		}
	}
	if n > 0 {
		x.mark = max(x.mark, g.itemIDs[n-1])
	}
	if validate {
		x.ids = slices.DeleteFunc(x.ids, func(sid entity.EntityID) bool { return g.stacks.Ref(sid) == nil })
	}
	return x.ids
}

// apparelKinds is the distinct kinds in the apparel index (a superset of
// those lying on the map), taken again whenever apparelGen moves.
func (g *Game) apparelKinds() []item.Kind {
	ids := g.apparelStock(false)
	x := g.apparelIdx()
	if x.kindsGen == x.gen+1 {
		return x.kinds
	}
	x.kinds, x.kindsGen = x.kinds[:0], x.gen+1
	for _, sid := range ids {
		if sp := g.stacks.Ref(sid); sp != nil && !slices.Contains(x.kinds, sp.Kind) {
			x.kinds = append(x.kinds, sp.Kind)
		}
	}
	return x.kinds
}
