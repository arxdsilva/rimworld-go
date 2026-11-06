package sense

import (
	"slices"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The Gear tab's own buttons (GEAR-004, 005, 007): Drop puts a thing at the
// pawn's feet now, as vanilla does (Unload, unload.go, hauls it to storage);
// Eat takes a pack entry through the ordinary eating or dose job; the lock
// forces a worn garment, so an apparel policy leaves it on. Each is one player
// command and no threshold drives it, so nothing here can flap. A refused
// command does nothing; DropWhy and EatWhy give its reason to the read model,
// so the tab greys the button out with the reason as its tip.

// Side is who a pawn is to the Gear tab's orders.
type Side uint8

const (
	SideOther    Side = iota // a guest, visitor, trader or hostile: refused
	SideColonist             // every button
	SidePrisoner             // drop only, at once: a captive takes no job
)

// side is id's side, and whether it stands (alive, not downed).
func (g Gear) side(id entity.EntityID) (Side, bool) {
	s := SideOther
	switch {
	case g.r.Prisoner(id):
		s = SidePrisoner
	case slices.Contains(g.r.PawnIDs(), id): // the colonists on this map
		s = SideColonist
	}
	return s, g.r.Up(id)
}

// GearOrders is what the Gear tab's commands ask of the game.
type GearOrders interface {
	// Up: id is alive and not downed. Prisoner: id is the colony's captive.
	Up(id entity.EntityID) bool
	Prisoner(id entity.EntityID) bool
	// Outfit is what id wears; SetOutfit stores it back (the forced marks).
	Outfit(id entity.EntityID) craft.Outfit
	SetOutfit(id entity.EntityID, o craft.Outfit)
	// MayDose: id's drug policy lets it take k now.
	MayDose(id entity.EntityID, k item.Kind) bool
	// Breaking: id is in a mental break, and takes no gear orders until it
	// passes.
	Breaking(id entity.EntityID) bool
}

// ModeAny is DropWhy's mode for the pawn alone: may anything of it be dropped.
const ModeAny = -1

const (
	// dropWearWait is how long taking a garment off to drop it takes: the
	// unload's pause and its garment's (ours, 50 ticks, under a second).
	dropWearWait = unloadWait + apparelWait
	// doseTicks is game/dose's time to take a dose (its ingestTicks).
	doseTicks = 120
	// tabFullAt is the food bar from which Eat is refused: 90 % (ours), when
	// almost anything eaten would go to waste.
	tabFullAt = needs.Full * 9 / 10
)

// The reasons a button is refused (our own words).
const (
	WhyNotOurs   = "Not under your command"
	WhyDown      = "Down or dead"
	WhyGone      = "Nothing there to use"
	WhyBusy      = "Can't start that now"
	WhyFull      = "Too full to eat"
	WhyPolicy    = "The drug policy forbids it"
	WhyInedible  = "Can't be eaten"
	WhyCaptive   = "Prisoners don't take orders"
	WhyNoGarment = "Not wearing that"
	WhyBreak     = "In a mental break"
)

// DropWhy is why id may not drop the thing a DropGear names ("" it may);
// ModeAny asks about the pawn alone.
func (g Gear) DropWhy(id entity.EntityID, mode, index int) string {
	switch s, up := g.side(id); {
	case s == SideOther:
		return WhyNotOurs
	case !up:
		return WhyDown
	case g.r.Breaking(id):
		return WhyBreak
	}
	ok := false
	switch mode {
	case ModeAny:
		ok = true
	case modePack:
		_, ok = entryOf(g.s.Packs[id], index)
	case modeApparel:
		_, ok = g.r.Worn(id, max(index, 0))
		ok = ok && index >= 0
	case modeWeapon:
		ok = g.r.Armed(id)
	case modeSpare:
		ok = g.r.HasSpare(id)
	}
	if !ok {
		return WhyGone
	}
	return ""
}

// Drop is command.DropGear, run now or when its turn in the order queue comes:
// a pack entry, the weapon or the spare falls at id's feet at once; a worn
// garment comes off first, a forced job of dropWearWait ticks (a prisoner's at
// once: it takes no job). It returns why it was refused, "" when done or begun.
func (g Gear) Drop(id entity.EntityID, mode, index int) string {
	if why := g.DropWhy(id, mode, index); why != "" || mode == ModeAny {
		return why
	}
	switch mode {
	case modePack:
		p := g.s.Packs[id]
		p.Items = slices.Clone(p.Items)
		e, _ := entryOf(p, index)
		st, _ := p.Take(e.ID, e.Stack.Qty)
		g.setPack(id, p)
		g.r.SetDown(id, st)
	case modeWeapon:
		g.r.DropWeapon(id)
	case modeSpare:
		g.r.DropSpare(id)
	case modeApparel:
		slot, _ := g.r.Worn(id, index)
		if s, _ := g.side(id); s == SidePrisoner {
			g.r.TakeOff(id, slot)
			return ""
		}
		g.r.Interrupt(id)
		x, y := g.r.Pos(id)
		if !g.r.StartJob(id, job.Record{Kind: job.UnloadPack, TX: x, TY: y, Count: modeApparel, BillIndex: slot, WorkLeft: dropWearWait, Forced: true}) {
			return WhyBusy
		}
	}
	return ""
}

// EatWhy is why id may not eat or take pack entry ("" it may): a colonist
// standing, the entry there, and food when the bar is below tabFullAt or a drug
// its policy allows. A diet does not stop it: the player chose.
func (g Gear) EatWhy(id entity.EntityID, entry uint32) string {
	switch s, up := g.side(id); {
	case s == SidePrisoner:
		return WhyCaptive
	case s != SideColonist:
		return WhyNotOurs
	case !up:
		return WhyDown
	case g.r.Breaking(id):
		return WhyBreak
	}
	e, ok := entryOf(g.s.Packs[id], int(entry))
	switch k := e.Stack.Kind; {
	case !ok:
		return WhyGone
	case k.IsDrug():
		if !g.r.MayDose(id, k) {
			return WhyPolicy
		}
	case !k.IsFood():
		return WhyInedible
	case g.r.Hunger(id) >= tabFullAt:
		return WhyFull
	}
	return ""
}

// Consume is command.ConsumeFromPack: id takes the entry out and eats it where
// it stands (job.Eat eats the portions that fill it best, and what is left
// goes back, BackToPack), or takes one dose of a drug (job.Ingest). Either is
// a forced job, so queued drops wait for it. It returns why it was refused.
func (g Gear) Consume(id entity.EntityID, entry uint32) string {
	if why := g.EatWhy(id, entry); why != "" {
		return why
	}
	e, _ := entryOf(g.s.Packs[id], int(entry))
	drug, n := e.Stack.Kind.IsDrug(), item.CarryLimit
	if drug {
		n = 1
	}
	g.r.Interrupt(id)
	stack, ok := g.TakeOut(id, entry, n)
	if !ok {
		return WhyBusy
	}
	x, y := g.r.Pos(id)
	rec := job.Record{Kind: job.Eat, Target: stack, Carry: stack, TX: x, TY: y, WorkLeft: needs.EatWorkTicks, Count: 2, Forced: true}
	if drug {
		rec = job.Record{Kind: job.Ingest, Target: stack, Carry: stack, TX: x, TY: y, WorkLeft: doseTicks, Forced: true}
	}
	if !g.r.StartJob(id, rec) {
		if !g.BackToPack(id, stack) {
			g.r.Lay(id, stack)
		}
		g.r.SetJob(id, job.Record{})
		return WhyBusy
	}
	j := g.r.Job(id)
	j.Carry = stack // as eatFromPack: set down uneaten, it goes back
	g.r.SetJob(id, j)
	return ""
}

// Lock is command.SetApparelForced: the n-th garment a colonist wears (the
// read model's order, as Drop's) is forced (on) or released.
func (g Gear) Lock(id entity.EntityID, n int, on bool) string {
	if s, _ := g.side(id); s != SideColonist {
		return WhyNotOurs
	}
	if g.r.Breaking(id) {
		return WhyBreak
	}
	slot, ok := g.r.Worn(id, max(n, 0))
	if !ok || n < 0 {
		return WhyNoGarment
	}
	o := g.r.Outfit(id)
	o.SetForced(item.Slot(slot), on)
	g.r.SetOutfit(id, o)
	return ""
}

// PackGear is a pack entry's row facts for the tab (PackItemView.Gear).
func (g Gear) PackGear(id entity.EntityID, e inventory.Entry) gearview.Pack {
	use := gearview.UseNone
	switch k := e.Stack.Kind; {
	case k.IsDrug():
		use = gearview.UseDrug
	case k.IsFood():
		use = gearview.UseEat
	}
	return gearview.Pack{Grams: e.Stack.Kind.MassGrams() * e.Stack.Qty, Use: use, Why: g.EatWhy(id, e.ID)}
}

// entryOf is the entry of p with ID id.
func entryOf(p inventory.Pack, id int) (inventory.Entry, bool) {
	i := slices.IndexFunc(p.Items, func(e inventory.Entry) bool { return int(e.ID) == id })
	if id < 0 || i < 0 {
		return inventory.Entry{}, false
	}
	return p.Items[i], true
}

// applyGear runs the Gear tab's three commands; a refused one does nothing.
func (g Gear) applyGear(kind command.Kind, pawn entity.EntityID, mode, index int, on bool) {
	switch kind {
	case command.DropGear:
		g.Drop(pawn, mode, index)
	case command.ConsumeFromPack:
		g.Consume(pawn, uint32(max(index, 0)))
	case command.SetApparelForced:
		g.Lock(pawn, index, on)
	}
}
