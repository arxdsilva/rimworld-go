package craft

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/exposure"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

// Worn apparel (PRD-2): what a pawn has on is kept piece by piece, with
// each garment's material, quality and wear, so a good parka stays good
// on and off the body and wears out while worn.

// Piece is one garment a pawn wears (Kind 0: the slot is bare).
type Piece struct {
	Kind    item.Kind
	Stuff   item.Kind // 0: its kind's default material
	Quality uint8     // quality.Stored (0: none, read as Normal)
	Wear    int       // hit points lost
	// Of is the pawn it was on when that pawn died (tainted; 0: clean).
	Of entity.EntityID
}

// Slots is how many slots an Outfit holds, indexed by item.Slot:
// NoSlot (never a garment), Body, Head, Legs.
const Slots = 4

// Outfit is what one pawn wears, by slot, and the slots the player chose
// ("Wear X"), one bit per slot.
type Outfit struct {
	On     [Slots]Piece
	Forced uint8
}

// OutfitSave is an Outfit in the save: a slice, so later slots load
// into older saves' outfits and back.
type OutfitSave struct {
	On     []Piece
	Forced uint8
}

// PieceOf is the garment stack st becomes when worn (one item of it).
func PieceOf(st item.Stack) Piece {
	return Piece{Kind: st.Kind, Stuff: st.Stuff, Quality: st.Quality, Wear: st.Wear, Of: st.Of}
}

// Stack is the garment as a one-item stack at (x, y), as it falls when
// taken off.
func (p Piece) Stack(x, y int) item.Stack {
	return item.Stack{Kind: p.Kind, Qty: 1, X: x, Y: y, Stuff: p.Stuff, Quality: p.Quality, Wear: p.Wear, Of: p.Of}
}

// HP and MaxHP are the garment's hit points left and when new.
func (p Piece) HP() int    { return p.Stack(0, 0).HP() }
func (p Piece) MaxHP() int { return p.Stack(0, 0).MaxHP() }

// Insulation is the garment's cold and heat insulation (tenths of a
// degree), by its kind, material and quality.
func (p Piece) Insulation() (cold, heat int) { return item.Insulation(p.Kind, p.Stuff, p.Quality) }

// Kind is the garment kind in slot s (0: bare).
func (o Outfit) Kind(s item.Slot) item.Kind {
	if int(s) >= Slots || s == item.NoSlot {
		return 0
	}
	return o.On[s].Kind
}

// IsForced reports that the player chose what is worn in slot s.
func (o Outfit) IsForced(s item.Slot) bool { return o.Forced&(1<<s) != 0 }

// SetForced marks slot s the player's (on) or the colonist's own.
func (o *Outfit) SetForced(s item.Slot, on bool) {
	if on {
		o.Forced |= 1 << s
	} else {
		o.Forced &^= 1 << s
	}
}

// hitWearPer is the damage reaching a garment that costs it one hit
// point. wiki: Apparel (1 HP per 4 damage, deflected or halved alike).
// original tuning: the share rounds to the nearest hit point.
const hitWearPer = 4

// HitWear wears the garment in slot s by a hit that reached it with dmg
// (CBT-1): one hit point per hitWearPer, so ratty and tattered follow
// from its hit points. A garment left with none is destroyed: the slot
// empties, the player's choice for it is dropped, and its kind is
// returned (0: it holds).
func (o *Outfit) HitWear(s item.Slot, dmg int) item.Kind {
	if int(s) >= Slots || o.On[s].Kind == 0 || dmg <= 0 {
		return 0
	}
	p := &o.On[s]
	p.Wear += (dmg + hitWearPer/2) / hitWearPer
	if p.MaxHP() == 0 || p.HP() > 0 {
		return 0
	}
	k := p.Kind
	*p = Piece{}
	o.SetForced(s, false)
	return k
}

// Taint marks everything in o as worn by pawn of when it died.
func (o Outfit) Taint(of entity.EntityID) Outfit {
	for s := range o.On {
		if o.On[s].Kind != 0 {
			o.On[s].Of = of
		}
	}
	return o
}

// Legacy is an outfit from an older save's kinds-only set: index
// item.NoSlot holds the forced bits, the rest a kind per slot.
func Legacy(w [3]item.Kind) Outfit {
	o := Outfit{Forced: uint8(w[item.NoSlot])}
	for s := item.Body; int(s) < len(w); s++ {
		o.On[s].Kind = w[s]
	}
	return o
}

// ExportOutfits is the save form of every outfit with anything in it
// (nil for none).
func ExportOutfits(m map[entity.EntityID]Outfit) map[entity.EntityID]OutfitSave {
	var out map[entity.EntityID]OutfitSave
	for id, o := range m {
		if o == (Outfit{}) {
			continue
		}
		if out == nil {
			out = make(map[entity.EntityID]OutfitSave, len(m))
		}
		out[id] = OutfitSave{On: append([]Piece(nil), o.On[:]...), Forced: o.Forced}
	}
	return out
}

// ImportOutfits restores ExportOutfits' outfits; legacy is an older
// save's kinds-only sets, read only when saved is nil.
func ImportOutfits(saved map[entity.EntityID]OutfitSave, legacy map[entity.EntityID][3]item.Kind) map[entity.EntityID]Outfit {
	out := make(map[entity.EntityID]Outfit, max(len(saved), len(legacy)))
	if saved == nil {
		for id, w := range legacy {
			out[id] = Legacy(w)
		}
		return out
	}
	for id, s := range saved {
		o := Outfit{Forced: s.Forced}
		copy(o.On[:], s.On)
		out[id] = o
	}
	return out
}

// Tainted apparel mood by how many tainted pieces are worn (wiki:
// Apparel): 1 -5, 2 -8, 3 -11, 4 or more -14.
var taintedMood = [...]int{0, -5, -8, -11, -14}

// TaintedMood is the mood of wearing n tainted pieces.
func TaintedMood(n int) int { return taintedMood[min(max(n, 0), len(taintedMood)-1)] }

// AppendThoughts appends the outfit's mood thoughts to ts: the worst
// piece's wear (ratty or tattered, wiki: Apparel) and the tainted
// pieces. A pawn in fine, clean clothes appends nothing.
func (o Outfit) AppendThoughts(ts []mood.Thought) []mood.Thought {
	worst, tainted := item.Fine, 0
	for _, p := range o.On {
		if p.Kind == 0 {
			continue
		}
		if p.Of != 0 {
			tainted++
		}
		worst = max(worst, item.StageOf(p.HP(), p.MaxHP()))
	}
	switch worst {
	case item.Tattered:
		ts = appendThought(ts, mood.TatteredApparel, 0)
	case item.Ratty:
		ts = appendThought(ts, mood.RattyApparel, 0)
	}
	if tainted > 0 {
		ts = appendThought(ts, mood.TaintedApparel, TaintedMood(tainted))
	}
	return ts
}

// appendThought appends thought k with its def's value, or v when set.
func appendThought(ts []mood.Thought, k mood.Kind, v int) []mood.Thought {
	d := mood.DefOf(k)
	if v == 0 {
		v = d.Value
	}
	return append(ts, mood.Thought{Kind: k, Label: d.Label, Value: v})
}

// WornView is one worn garment as the inspector shows it.
type WornView struct {
	Kind, Stuff, Quality string // Stuff and Quality "" when it has none
	HP, MaxHP            int
	Tainted              bool
	Ratty, Tattered      bool
}

// Views lists the outfit's garments in slot order (nil: none on).
func (o Outfit) Views() []WornView {
	var out []WornView
	for _, p := range o.On {
		if p.Kind == 0 {
			continue
		}
		hp, mx := p.HP(), p.MaxHP()
		st := item.StageOf(hp, mx)
		out = append(out, WornView{Kind: p.Kind.String(), Stuff: StuffLabel(p.Kind, p.Stuff), Quality: QualityLabel(p.Quality),
			HP: hp, MaxHP: mx, Tainted: p.Of != 0, Ratty: st == item.Ratty, Tattered: st == item.Tattered})
	}
	return out
}

// QualityLabel names a stored quality ("" for none).
func QualityLabel(q uint8) string {
	if l, ok := quality.FromStored(q); ok {
		return l.String()
	}
	return ""
}

// StuffLabel names the material product k is made of ("" for a product
// that takes none).
func StuffLabel(k, stuff item.Kind) string {
	if item.MadeFrom(k) == 0 {
		return ""
	}
	return item.Stack{Kind: k, Stuff: stuff}.StuffOfStack().String()
}

// Bands is the comfortable range of a pawn in everyday clothes with o on
// top: each garment's insulation by its material and quality.
func (o Outfit) Bands() exposure.Bands {
	b := exposure.Everyday
	for _, p := range o.On[item.Body:] {
		if p.Kind != 0 {
			c, h := p.Insulation()
			b = b.Wearing(c, h)
		}
	}
	return b
}

// DaySpan is the spread of the day's air around its mean (tenths of a
// degree C) an outfit is judged against: the day, not the hour, so
// clothes don't change at dawn.
const DaySpan = 100

// DayFit scores bands b for the day: how much of its likely range (mean
// ± DaySpan) b leaves uncomfortable. Lower is better.
func DayFit(b exposure.Bands, mean int) int {
	lo, hi := mean-DaySpan, mean+DaySpan
	miss := 0
	if lo < b.Min {
		miss += b.Min - lo
	}
	if hi > b.Max {
		miss += hi - b.Max
	}
	return miss
}

// DaySurplus is how far b reaches past the day's range: insulation the
// day does not need.
func DaySurplus(b exposure.Bands, mean int) int {
	return max(0, mean-DaySpan-b.Min) + max(0, b.Max-mean-DaySpan)
}
