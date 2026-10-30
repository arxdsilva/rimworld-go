package zone

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/world/fire"
)

// Special is a storage zone's special filters (PRD-4, STO-2). The zero
// value takes everything: each toggle is stored as its refusal, so older
// saves and zones that never had one keep taking what they did.
type Special struct {
	// NoRotten refuses spoiled stacks; NoTainted apparel a dead body wore.
	NoRotten, NoTainted bool
	// HPMin..HPMax is the hit-point range in percent, for kinds with hit
	// points (HPMax 0: 100).
	HPMin, HPMax uint8
	// QualityMin..QualityMax is the quality range, for kinds with a
	// quality (QualityMax is stored one up: 0 is Legendary).
	QualityMin, QualityMax uint8
	// STO-2, after the vanilla special filters: NoFresh refuses perishable
	// stacks not yet spoiled; NoClean apparel nobody died in (the
	// non-deadman's apparel); NoSmeltable and NoNonSmeltable what a
	// smelter does or doesn't take back to steel; NoBurnable and
	// NoNonBurnable what does or doesn't catch fire. NoColonist and
	// NoStranger are the corpse filters graves and sarcophagi use for a
	// colonist's or anyone else's body (TakesBody; stockpiles ignore them).
	NoFresh, NoClean            bool
	NoSmeltable, NoNonSmeltable bool
	NoBurnable, NoNonBurnable   bool
	NoColonist, NoStranger      bool
}

// Special filter indexes (SetStockpileSpecial's Index): the toggles
// (Enabled allows) and the two ranges (low..high). Append only: the UI
// sends them.
const (
	SpecialRotten = iota
	SpecialTainted
	SpecialHP
	SpecialQuality
	SpecialFresh
	SpecialClean
	SpecialSmeltable
	SpecialNonSmeltable
	SpecialBurnable
	SpecialNonBurnable
	SpecialColonist
	SpecialStranger
	numSpecials
)

// Toggle is one special filter switch for the settings window: its
// SetStockpileSpecial Index, its name (as Special.Names lists it when
// on) and Grave, a corpse filter only graves and sarcophagi use.
type Toggle struct {
	Index int
	Name  string
	Grave bool
}

// Toggles lists every special filter switch in display order.
func Toggles() []Toggle {
	return []Toggle{
		{SpecialRotten, "allow rotten", false}, {SpecialFresh, "allow fresh", false},
		{SpecialTainted, "allow tainted apparel", false}, {SpecialClean, "allow non-deadman's apparel", false},
		{SpecialSmeltable, "allow smeltable", false}, {SpecialNonSmeltable, "allow non-smeltable", false},
		{SpecialBurnable, "allow burnable", false}, {SpecialNonBurnable, "allow non-burnable", false},
		{SpecialColonist, "allow colonist corpses", true}, {SpecialStranger, "allow stranger corpses", true},
	}
}

// SpecialNames are the first two toggles by index (SpecialRotten,
// SpecialTainted), as PRD-4's settings window lists them; Toggles has
// them all.
func SpecialNames() []string { return []string{"allow rotten", "allow tainted apparel"} }

// refuses is toggle i's refusal flag in s (nil for a range or an
// unknown index).
func (s *Special) refuses(i int) *bool {
	switch i {
	case SpecialRotten:
		return &s.NoRotten
	case SpecialTainted:
		return &s.NoTainted
	case SpecialFresh:
		return &s.NoFresh
	case SpecialClean:
		return &s.NoClean
	case SpecialSmeltable:
		return &s.NoSmeltable
	case SpecialNonSmeltable:
		return &s.NoNonSmeltable
	case SpecialBurnable:
		return &s.NoBurnable
	case SpecialNonBurnable:
		return &s.NoNonBurnable
	case SpecialColonist:
		return &s.NoColonist
	case SpecialStranger:
		return &s.NoStranger
	}
	return nil
}

// HPRange is the allowed hit points in percent.
func (s Special) HPRange() (lo, hi int) {
	hi = 100
	if s.HPMax > 0 {
		hi = int(s.HPMax)
	}
	return int(s.HPMin), hi
}

// QualityRange is the allowed quality levels.
func (s Special) QualityRange() (lo, hi quality.Level) {
	hi = quality.Legendary
	if s.QualityMax > 0 {
		hi = quality.Level(s.QualityMax - 1)
	}
	return quality.Level(s.QualityMin), hi
}

// Set applies special filter i: a toggle allows (on) or refuses; a range
// takes lo..hi (percent for hit points, quality levels for quality),
// clamped and ordered. An unknown index changes nothing.
func (s Special) Set(i int, on bool, lo, hi int) Special {
	if p := s.refuses(i); p != nil {
		*p = !on
		return s
	}
	switch i {
	case SpecialHP:
		lo, hi = clampRange(lo, hi, 0, 100)
		s.HPMin, s.HPMax = uint8(lo), uint8(max(hi, 1))
		if hi == 100 {
			s.HPMax = 0
		}
	case SpecialQuality:
		lo, hi = clampRange(lo, hi, 0, int(quality.Legendary))
		s.QualityMin, s.QualityMax = uint8(lo), uint8(hi+1)
		if hi == int(quality.Legendary) {
			s.QualityMax = 0
		}
	}
	return s
}

func clampRange(lo, hi, floor, ceil int) (int, int) {
	lo, hi = min(max(lo, floor), ceil), min(max(hi, floor), ceil)
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo, hi
}

// Names lists the toggles that are on (rotten and tainted first, as
// SpecialNames, then the rest of Toggles in order), then each range the
// player narrowed ("hp 50-100%", "quality 3-6": quality levels, 0 awful
// to 6 legendary).
func (s Special) Names() []string {
	var out []string
	for _, i := range []int{SpecialRotten, SpecialTainted} {
		if !*s.refuses(i) {
			out = append(out, SpecialNames()[i])
		}
	}
	for _, t := range Toggles() {
		if t.Index != SpecialRotten && t.Index != SpecialTainted && !*s.refuses(t.Index) {
			out = append(out, t.Name)
		}
	}
	if lo, hi := s.HPRange(); lo > 0 || hi < 100 {
		out = append(out, "hp "+itoa(lo)+"-"+itoa(hi)+"%")
	}
	if lo, hi := s.QualityRange(); lo > quality.Awful || hi < quality.Legendary {
		out = append(out, "quality "+itoa(int(lo))+"-"+itoa(int(hi)))
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}
	return string(b[i:])
}

// Takes reports stack st passes the special filters (the kind is checked
// by Allows; the corpse filters by TakesBody).
func (s Special) Takes(st item.Stack) bool {
	if s.NoColonist || s.NoStranger {
		s.NoColonist, s.NoStranger = false, false
	}
	if s == (Special{}) {
		return true
	}
	rotten := st.Rotten()
	if s.NoRotten && rotten || s.NoFresh && st.Kind.Perishable() && !rotten {
		return false
	}
	if tainted := st.Tainted(); s.NoTainted && tainted || s.NoClean && st.Kind.IsApparel() && !tainted {
		return false
	}
	if s.NoSmeltable || s.NoNonSmeltable {
		if smelt := recipe.SmeltYield(st) > 0; s.NoSmeltable && smelt || s.NoNonSmeltable && !smelt {
			return false
		}
	}
	if s.NoBurnable || s.NoNonBurnable {
		if burn := fire.ItemFlammability(st.Kind) > 0; s.NoBurnable && burn || s.NoNonBurnable && !burn {
			return false
		}
	}
	if mhp := st.MaxHP(); mhp > 0 {
		lo, hi := s.HPRange()
		if pct := st.HP() * 100 / mhp; pct < lo || pct > hi {
			return false
		}
	}
	if l, ok := quality.FromStored(st.Quality); ok && item.HasQuality(st.Kind) {
		lo, hi := s.QualityRange()
		if l < lo || l > hi {
			return false
		}
	}
	return true
}

// TakesBody reports a grave or sarcophagus with these filters takes body
// st: the special filters, and for a human corpse or bones the colonist
// or stranger filter (colonist: the dead was one of the colony).
func (s Special) TakesBody(st item.Stack, colonist bool) bool {
	if sets.bodies.Has(st.Kind) && (colonist && s.NoColonist || !colonist && s.NoStranger) {
		return false
	}
	return s.Takes(st)
}
