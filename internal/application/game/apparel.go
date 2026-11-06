package game

import (
	"slices"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/application/game/outfit"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Clothes: how colonists dress lives in game/outfit (PLAY-017, TEMP-4,
// GEAR-3); this file is its adapters over the game and the entry points
// the rest of the package calls.

// worn is what a colonist has on, piece by piece with each garment's
// material, quality and wear, and the slots the player chose
// (craft/garment.go, PRD-2).
type worn = craft.Outfit

// wearSlots are the slots garments go in, in slot order (the views').
var wearSlots = outfit.Slots

// apparelIndex is a map's free-clothes index (game/outfit), never saved.
type apparelIndex = outfit.Index

// outfitOf is game/outfit's dressing over the current map.
func outfitOf(g *Game) outfit.Dresser {
	return outfit.New(&g.clothes, outfitBook(g), outfit.Roles{Wardrobe: outfitWardrobe{g}, Stock: outfitStock{g}, Weather: outfitWeather{g}, Walks: outfitWalks{g}})
}

// outfitBook is the colony's apparel policies (game/outfit, GEAR-3).
func outfitBook(g *Game) *outfit.Book { return &g.worldS().outfits }

// applyOutfit runs SetApparelPolicy, EditApparelPolicy and
// ClearForcedApparel (tick.go).
func applyOutfit(g *Game, c command.Command) {
	if c.Kind == command.ClearForcedApparel {
		outfitOf(g).ClearForced(entity.EntityID(c.Pawn))
		return
	}
	outfitBook(g).Apply(c, outfitWearers{g})
}

// bandsOf is a colonist's comfortable range given what it wears.
func (g *Game) bandsOf(id entity.EntityID) exposure.Bands {
	c, h := g.comfortShift(id)
	return shifted(g.apparel[id].Bands(), c, h) // craft/garment.go
}

// shifted is b moved by a colonist's comfort shift (game/outfit).
func shifted(b exposure.Bands, cold, heat int) exposure.Bands { return outfit.Shifted(b, cold, heat) }

// wear puts on the garment stack sid (one item), dropping whatever was in
// that slot; forced: the player ordered it, and the slot becomes theirs.
func (g *Game) wear(id, sid entity.EntityID, st item.Stack, forced bool) {
	outfitOf(g).Wear(id, sid, st, forced)
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

// dressChanged starts slot s's cooldown after a change.
func (g *Game) dressChanged(id entity.EntityID, s item.Slot) { outfitOf(g).Changed(id, s) }

// noteApparel adds new apparel stack id to the free-clothes index.
func (g *Game) noteApparel(id entity.EntityID) { g.clothes.Note(id) }

// assignWear is the dress check of free time (tick.go: non-work blocks,
// and when no work was found).
func (g *Game) assignWear(id entity.EntityID) bool { return g.dressCheck(id) }

// dressCheck changes colonist id into a better outfit before it looks for
// work (PLAY-017); true when it set off for a garment.
func (g *Game) dressCheck(id entity.EntityID) bool { return outfitOf(g).Check(id) }

// dressInterrupt drops colonist id's interruptible work job j for a better
// outfit (PLAY-017; tick.go).
func (g *Game) dressInterrupt(id entity.EntityID, j job.Record) { outfitOf(g).Interrupt(id, j) }

// outfitView adds the apparel policy to v (PawnView.Gear, OUT-003).
func outfitView(g *Game, id entity.EntityID, v gearview.Pawn) gearview.Pawn {
	return outfitBook(g).View(id, v)
}

// outfitWorld is WorldView.Gear: the apparel policies (OUT-001).
func outfitWorld(g *Game) gearview.World { return gearview.World{Policies: outfitBook(g).Views()} }

// game/outfit's roles over the game: pointer-shaped, so building a
// Dresser allocates nothing.
type (
	outfitWardrobe struct{ g *Game }
	outfitStock    struct{ g *Game }
	outfitWeather  struct{ g *Game }
	outfitWalks    struct{ g *Game }
	outfitWearers  struct{ g *Game }
)

// Wearer: a living colonist of the colony, not a guest or a raider.
func (h outfitWearers) Wearer(id entity.EntityID) bool {
	v, ok := h.g.vital.Get(id)
	return ok && v.Status != health.Dead && h.g.colonist(id) && !h.g.visitor(id)
}

// Wearers is everyone dressed who is neither a raider nor a guest, on
// any map or away (the apparel map is colony-wide), in id order.
func (h outfitWearers) Wearers() []entity.EntityID {
	var out []entity.EntityID
	for id := range h.g.apparel {
		if !h.g.isRaider(id) && !h.g.visitor(id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func (h outfitWardrobe) Worn(id entity.EntityID) craft.Outfit       { return h.g.apparel[id] }
func (h outfitWardrobe) SetWorn(id entity.EntityID, w craft.Outfit) { h.g.apparel[id] = w }
func (h outfitWardrobe) Drop(st item.Stack, x, y int)               { h.g.dropStackOf(st, x, y) }
func (h outfitWardrobe) Drafted(id entity.EntityID) bool            { return h.g.drafted(id) }
func (h outfitWardrobe) Note(id entity.EntityID, putOn bool, k item.Kind) {
	kind := uint8(logTookOff)
	if putOn {
		kind = logPutOn
	}
	h.g.dressNote(id, kind, k)
}
func (h outfitWardrobe) Use(sid entity.EntityID, st item.Stack) {
	if st.Qty <= 1 {
		h.g.deleteItem(sid)
	} else {
		st.Qty--
		h.g.putStack(sid, st)
	}
}

func (h outfitStock) ItemIDs() []entity.EntityID            { return h.g.itemIDs }
func (h outfitStock) Stack(sid entity.EntityID) *item.Stack { return h.g.stacks.Ref(sid) }
func (h outfitStock) Owner(sid entity.EntityID) (entity.EntityID, bool) {
	return h.g.reserve.EntityOwner(sid)
}

func (h outfitWeather) Now() uint64                                 { return uint64(h.g.clock.Now()) }
func (h outfitWeather) DailyMean() int                              { return int(h.g.climate.DailyMean(h.g.clock.Now())) }
func (h outfitWeather) Comfort(id entity.EntityID) (cold, heat int) { return h.g.comfortShift(id) }
func (h outfitWeather) TempAt(x, y int) int                         { return int(h.g.temperatureAt(x, y)) }

func (h outfitWalks) Pos(id entity.EntityID) (x, y int) {
	p, _ := h.g.positions.Get(id)
	return p.X, p.Y
}
func (h outfitWalks) Postponed(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int) bool {
	return h.g.postponedFor(id, kind, target, x, y)
}
func (h outfitWalks) PutOff(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int, until uint64) {
	h.g.putOff(postKey{Pawn: id, Kind: kind, Target: target, X: x, Y: y}, until)
}
func (h outfitWalks) Allowed(id entity.EntityID, x, y int) bool   { return h.g.areaAllows(id, x, y) }
func (h outfitWalks) Reachable(id entity.EntityID, x, y int) bool { return h.g.reachable(id, x, y) }
func (h outfitWalks) StartJob(id entity.EntityID, rec job.Record) bool {
	return h.g.startJob(id, rec)
}
func (h outfitWalks) Job(id entity.EntityID) job.Record { j, _ := h.g.jobs.Get(id); return j }
func (h outfitWalks) Interrupt(id entity.EntityID)      { h.g.interrupt(id, job.Interrupted) }
