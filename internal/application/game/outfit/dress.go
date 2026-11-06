package outfit

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

// Clothes. Each colonist wears at most one garment per layer slot
// (TEMP-4: shirt and pants on the skin, a middle layer, the outer layer,
// headgear, flak pants and a belt; craft/dress.go), never two that share
// a layer over the same region; garments widen its comfortable
// temperature band. A colonist dresses in the best outfit it can get for
// the day (PLAY-017): before work at every job pick, and in the middle of plain
// work when a better one turns up. Pieces it should not wear come off at
// once; then it walks to each new piece in turn. A slot that just changed
// stays as it is for Cool (unless the colonist is freezing or
// sweltering), and a slot the player dressed ("Wear X") stays the
// player's until another garment is ordered for it.

const (
	// Gain: a change must spare at least 1 C of discomfort.
	Gain = 10
	// Cool is how long a slot that changed stays as it is.
	Cool = 2 * clock.TicksPerHour
	// Every: a colonist at work looks at its clothes once per this many
	// ticks (staggered by id).
	Every = 60
	// tries caps the garments one check sets off for.
	tries = 3
	// maxOpts sizes the per-slot option buffers (keep, bare and one per
	// apparel kind of the busiest slot, the outer layer; more only allocate).
	maxOpts = 10
)

// Slots are the slots garments go in, in slot order (the views').
var Slots = [...]item.Slot{item.Body, item.Head, item.Legs, item.Belt, item.SkinTorso, item.SkinLegs, item.Middle} // CBT-3: belts only by order

// Shifted is b moved by a colonist's comfort shift: cold is added to the
// floor, heat to the ceiling (skills.go: build and traits, Cold-hardy).
func Shifted(b exposure.Bands, cold, heat int) exposure.Bands {
	if cold != 0 || heat != 0 {
		b = b.Wearing(-cold, heat)
	}
	return b
}

// Dresser is dressing over the game's roles, a map's clothes index and
// the colony's apparel policies. It holds no state of its own: build one
// where it is needed.
type Dresser struct {
	idx  *Index
	book *Book // nil: the defaults
	r    Roles
}

// New is a Dresser over idx (the current map's), book and the roles.
func New(idx *Index, book *Book, r Roles) Dresser { return Dresser{idx: idx, book: book, r: r} }

// policy is colonist id's apparel policy.
func (d Dresser) policy(id entity.EntityID) Policy {
	if d.book == nil {
		return defaults[Anything]
	}
	return d.book.Of(id)
}

// shed is the first slot (in Slots order) whose garment policy p refuses
// and the colonist may take off now (ok false: none): one slot per check, so a
// policy change undresses a colonist piece by piece, each slot then held
// for Cool like any change (OUT-004). Forced slots never are.
func shed(cur craft.Outfit, locked [craft.Slots]bool, p Policy) (item.Slot, bool) {
	for _, s := range Slots {
		if pc := cur.On[s]; pc.Kind != 0 && !locked[s] && !p.Takes(pc.Stack(0, 0)) {
			return s, true
		}
	}
	return 0, false
}

// Wear puts on the garment stack sid (one item), dropping whatever was in
// that slot; forced: the player ordered it, and the slot becomes theirs.
func (d Dresser) Wear(id, sid entity.EntityID, st item.Stack, forced bool) {
	x, y := d.r.Walks.Pos(id)
	w := d.r.Wardrobe.Worn(id)
	slot, off := st.Kind.Slot(), w.Conflicts(st.Kind) // craft/garment.go: its slot, and a shell's or one-piece's
	for _, s := range Slots {
		if old := w.On[s]; off.Has(s) {
			d.r.Wardrobe.Drop(old.Stack(x, y), x, y)
			w.On[s] = craft.Piece{}
			w.SetForced(s, false)
			d.Changed(id, s)
			if s != slot {
				d.r.Wardrobe.Note(id, false, old.Kind)
			}
		}
	}
	w.On[slot] = craft.PieceOf(st)
	w.SetForced(slot, forced)
	d.r.Wardrobe.SetWorn(id, w)
	d.r.Wardrobe.Use(sid, st)
	d.Changed(id, slot)
	d.r.Wardrobe.Note(id, true, st.Kind)
}

// Plan is colonist id's best outfit for the day, and whether it is worth
// changing into: covered where it is naked, or at least Gain less
// discomfort than what it has on, judged with its comfort shift on both
// sides. Slots the player chose, and slots in their cooldown, keep what
// they have; a garment its apparel policy refuses comes off (shed), and
// only garments the policy allows go on. With full false the plan is optimistic: every kind in the
// apparel index counts as lying at its feet. Garments only ever drop out
// of the full plan's choices, so when the optimistic plan finds nothing
// worth the change neither does the full one: the cheap gate busy
// colonists are checked with.
func (d Dresser) Plan(id entity.EntityID, full bool) (craft.Plan, bool) {
	cur := d.r.Wardrobe.Worn(id)
	locked := d.locked(id, cur)
	pol := d.policy(id)
	off, shedding := shed(cur, locked, pol)
	var buf [craft.Slots][maxOpts]craft.Option
	var opts [craft.Slots][]craft.Option
	for _, s := range Slots {
		if opts[s] = buf[s][:0]; s != off || !shedding {
			opts[s] = append(opts[s], craft.Option{Kind: cur.Kind(s)}) // keep it on
		}
		if cur.Kind(s) != 0 && !locked[s] {
			opts[s] = append(opts[s], craft.Option{}) // take it off
		}
	}
	if full {
		d.candidates(id, cur, locked, pol, &opts)
	} else {
		for _, k := range d.Kinds() {
			if s := k.Slot(); !locked[s] && k != cur.Kind(s) && pol.Kind(k) {
				opts[s] = append(opts[s], craft.Option{Kind: k})
			}
		}
	}
	mean := d.r.Weather.DailyMean()
	c, h := d.r.Weather.Comfort(id)
	best := craft.Best(&opts, Shifted(exposure.Everyday, c, h), mean)
	return best, shedding || cur.Naked() && !best.Naked || craft.DayFit(Shifted(cur.Bands(), c, h), mean)-best.Miss >= Gain
}

// candidates adds to opts, per slot, the nearest garment of each kind
// colonist id could go and put on: lying free (not carried, forbidden,
// reserved by another or put off), allowed by its policy p, inside its
// allowed area and reachable. Kinds it already wears and locked slots are
// skipped.
func (d Dresser) candidates(id entity.EntityID, cur craft.Outfit, locked [craft.Slots]bool, p Policy, opts *[craft.Slots][]craft.Option) {
	px, py := d.r.Walks.Pos(id)
	for _, sid := range d.Stock(true) {
		sp := d.r.Stock.Stack(sid)
		if sp == nil || sp.Qty <= 0 || sp.Carrier != 0 || sp.Forbidden || !p.Takes(*sp) { // PRD-2: a dead man's clothes only by order, or by a policy that allows them
			continue
		}
		s := sp.Kind.Slot()
		if locked[s] || sp.Kind == cur.Kind(s) {
			continue
		}
		dist := pathing.Dist(px, py, sp.X, sp.Y)
		i := slices.IndexFunc(opts[s], func(o craft.Option) bool { return o.Stack != 0 && o.Kind == sp.Kind })
		if i >= 0 && opts[s][i].Walk <= dist {
			continue // a nearer one of this kind (ids ascend: ties keep the first)
		}
		if owner, taken := d.r.Stock.Owner(sid); taken && owner != id {
			continue
		}
		if d.r.Walks.Postponed(id, job.Equip, sid, 0, 0) || !d.r.Walks.Allowed(id, sp.X, sp.Y) || !d.r.Walks.Reachable(id, sp.X, sp.Y) {
			continue
		}
		o := craft.Option{Kind: sp.Kind, Stack: sid, X: sp.X, Y: sp.Y, Walk: dist}
		if i >= 0 {
			opts[s][i] = o
		} else {
			opts[s] = append(opts[s], o)
		}
	}
}

// locked reports the slots colonist id keeps as they are: the player's
// choices, and slots in their cooldown unless it is freezing or
// sweltering where it stands.
func (d Dresser) locked(id entity.EntityID, cur craft.Outfit) [craft.Slots]bool {
	var locked [craft.Slots]bool
	urgent, known := false, false
	for _, s := range Slots {
		switch {
		case cur.IsForced(s):
			locked[s] = true
		case d.Cooling(id, s):
			if !known {
				urgent, known = d.urgent(id), true
			}
			locked[s] = !urgent
		}
	}
	return locked
}

// urgent reports a colonist with a Freezing or Sweltering thought
// (rooms.go thresholds).
func (d Dresser) urgent(id entity.EntityID) bool {
	x, y := d.r.Walks.Pos(id)
	c, h := d.r.Weather.Comfort(id)
	t, b := d.r.Weather.TempAt(x, y), Shifted(d.r.Wardrobe.Worn(id).Bands(), c, h)
	return t < b.Freezing() || t > b.Sweltering()
}

// coolKey is the postponed entry (saved) holding slot s after a change:
// an Equip with no target, at (-1, slot).
func coolKey(s item.Slot) (job.Kind, entity.EntityID, int, int) { return job.Equip, 0, -1, int(s) }

// Cooling reports slot s of colonist id in its cooldown.
func (d Dresser) Cooling(id entity.EntityID, s item.Slot) bool {
	k, t, x, y := coolKey(s)
	return d.r.Walks.Postponed(id, k, t, x, y)
}

// Changed starts slot s's cooldown after a change.
func (d Dresser) Changed(id entity.EntityID, s item.Slot) {
	k, t, x, y := coolKey(s)
	d.r.Walks.PutOff(id, k, t, x, y, d.r.Weather.Now()+Cool)
}

// undress takes off at once what colonist id has on and outfit o leaves
// bare, dropping it at its feet.
func (d Dresser) undress(id entity.EntityID, o *craft.Plan) {
	for _, s := range Slots {
		w := d.r.Wardrobe.Worn(id)
		p := w.On[s]
		k := p.Kind
		if k == 0 || o.Pick[s].Kind != 0 {
			continue
		}
		x, y := d.r.Walks.Pos(id)
		w.On[s] = craft.Piece{}
		w.SetForced(s, false)
		d.r.Wardrobe.SetWorn(id, w)
		d.r.Wardrobe.Drop(p.Stack(x, y), x, y)
		d.Changed(id, s)
		d.r.Wardrobe.Note(id, false, k)
	}
}

// Dress changes colonist id into its best outfit: unwanted pieces come off
// at once, then it sets off for the nearest new piece (a later check takes
// the next). A fetch that fails out of reach or out of area was put off by
// the job start, so the next best is tried. True when it set off.
func (d Dresser) Dress(id entity.EntityID) bool {
	for range tries {
		o, ok := d.Plan(id, true)
		if !ok {
			return false
		}
		d.undress(id, &o)
		n := o.Next()
		if n.Stack == 0 {
			return false
		}
		if d.r.Walks.StartJob(id, job.Record{Kind: job.Equip, Target: n.Stack, TX: n.X, TY: n.Y}) {
			return true
		}
		if d.r.Walks.Job(id).Fail != job.Unreachable {
			return false // reserved, or the path search deferred: next tick
		}
	}
	return false
}

// Check changes colonist id into a better outfit before it looks for
// work (PLAY-017); true when it set off for a garment.
func (d Dresser) Check(id entity.EntityID) bool {
	if _, ok := d.Plan(id, false); !ok {
		return false
	}
	return d.Dress(id)
}

// breaks reports work j that may be dropped for clothes: plain work, not
// a need, an order, or a load being carried.
func breaks(j job.Record) bool {
	if j.Forced || j.Carry != 0 {
		return false
	}
	switch j.Kind {
	case job.Haul, job.FillHopper, job.Construct, job.Deconstruct, job.Harvest, job.Cut, job.Mine, job.Smooth, job.Sow,
		job.Bill, job.Refuel, job.Repair, job.Research, job.OperateDrill, job.ScanGround, job.ScanMinerals, job.Clean, job.Bury, job.Roof, job.Unroof,
		job.Milk, job.Shear, job.ClearSnow, job.Uninstall, job.Install, job.OpenContainer, job.StockTank, job.NetTank:
		return true
	}
	return false
}

// Interrupt drops colonist id's interruptible work job j for a better
// outfit (PLAY-017), once per Every ticks. Pieces to take off come off
// without stopping the work; a piece to fetch abandons it the way
// drafting does.
func (d Dresser) Interrupt(id entity.EntityID, j job.Record) {
	if (d.r.Weather.Now()+uint64(id))%Every != 0 || !breaks(j) || d.r.Wardrobe.Drafted(id) {
		return
	}
	if _, ok := d.Plan(id, false); !ok {
		return
	}
	o, ok := d.Plan(id, true)
	if !ok {
		return
	}
	if o.Next().Stack == 0 {
		d.undress(id, &o)
		return
	}
	d.r.Walks.Interrupt(id)
	d.Dress(id)
}

// ClearForced releases every garment the player forced on colonist id
// (OUT-005): its policy may take them off from the next check on.
func (d Dresser) ClearForced(id entity.EntityID) bool {
	w := d.r.Wardrobe.Worn(id)
	if w.Forced == 0 {
		return false
	}
	for _, s := range Slots {
		w.SetForced(s, false)
	}
	d.r.Wardrobe.SetWorn(id, w)
	return true
}
