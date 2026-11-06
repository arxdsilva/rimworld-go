package outfit

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The starting apparel policies' ids (OUT-001; the contents are our own).
// Ids are never reused.
const (
	// Anything allows every garment in any material and condition, but
	// never a dead man's clothes: how colonists dressed before policies,
	// and the default for new colonists.
	Anything = iota
	// Worker: everyday clothes and utility packs, no armour or helmets,
	// nothing below half its hit points.
	Worker
	// Soldier: every garment, armour and helmets included, nothing below
	// half its hit points. Dressing still scores warmth; the policy only
	// keeps ratty clothes off a fighter.
	Soldier
	// Nudist: utility only (shield belts, smokepop packs); a colonist on it
	// sheds its clothes unless the player forced them.
	Nudist
	numDefaults
)

// nameMax is the longest policy name (the editor's field).
const nameMax = 24

// Policy is one apparel policy: the kinds it allows (the Apparel branch of
// the filter tree), the materials it refuses, and the special filters
// (hit points, quality, tainted). Kinds outside the Apparel branch are not
// a policy's to forbid.
type Policy struct {
	ID      int
	Name    string
	Allow   zone.Filter
	NoStuff zone.Filter  // refused materials (zero: every material)
	Special zone.Special // HP and quality ranges, NoTainted
}

// Kind reports the policy allows garment kind k.
func (p Policy) Kind(k item.Kind) bool { return !apparel.Has(k) || p.Allow.Has(k) }

// Takes reports the policy allows garment st: its kind, its material and
// the special filters.
func (p Policy) Takes(st item.Stack) bool {
	return p.Kind(st.Kind) && !p.NoStuff.Has(st.StuffOfStack()) && p.Special.Takes(st)
}

// apparel and defaults are worked out once and only read afterwards
// (dressing reads them for every garment it weighs).
var (
	apparel  = resolve("Apparel")
	defaults = makeDefaults()
)

func resolve(name string) zone.Filter {
	f, _ := zone.Resolve(name)
	return f
}

// Defaults are the starting policies, a fresh copy.
func Defaults() []Policy { return slices.Clone(defaults) }

func makeDefaults() []Policy {
	clean := zone.Special{NoTainted: true}
	sound := clean.Set(zone.SpecialHP, true, 50, 100)
	worker := apparel.AndNot(resolve("Apparel/Armor")).AndNot(resolve("Apparel/Headgear"))
	return []Policy{
		{ID: Anything, Name: "Anything", Allow: apparel, Special: clean},
		{ID: Worker, Name: "Worker", Allow: worker, Special: sound},
		{ID: Soldier, Name: "Soldier", Allow: apparel, Special: sound},
		{ID: Nudist, Name: "Nudist", Allow: resolve("Apparel/Utility"), Special: clean},
	}
}

// Wearers tells the book who may hold a policy.
type Wearers interface {
	// Wearer: id is a colonist who may hold an apparel policy.
	Wearer(id entity.EntityID) bool
	// Wearers lists everyone who may hold one, on any map or away.
	Wearers() []entity.EntityID
}

// Book is the colony's apparel policies and who holds which. The zero Book
// is the defaults with nobody assigned: everyone wears by the default
// policy (Anything until the player picks another).
type Book struct {
	pols []Policy                // nil: Defaults()
	next int                     // the next new policy's id (0: numDefaults)
	def  int                     // the default policy's id
	of   map[entity.EntityID]int // assigned policy ids
	// views caches Views until the next edit (never saved).
	views []gearview.Policy
}

// Policies lists the policies in order (shared: read only).
func (b *Book) Policies() []Policy {
	if b.pols == nil {
		return defaults
	}
	return b.pols
}

// Default is the id of the policy new colonists get.
func (b *Book) Default() int { return b.def }

func (b *Book) find(id int) int {
	return slices.IndexFunc(b.Policies(), func(p Policy) bool { return p.ID == id })
}

// Of is colonist id's policy: the one assigned, else the default.
func (b *Book) Of(id entity.EntityID) Policy {
	ps := b.Policies()
	if pid, ok := b.of[id]; ok {
		if i := b.find(pid); i >= 0 {
			return ps[i]
		}
	}
	if i := b.find(b.def); i >= 0 {
		return ps[i]
	}
	return ps[0]
}

// Apply runs a SetApparelPolicy or EditApparelPolicy command; it reports
// whether anything changed. Unknown policies, pawns that may hold none,
// deleting the default or the last policy, and empty names change nothing.
func (b *Book) Apply(c command.Command, w Wearers) bool {
	if c.Kind == command.SetApparelPolicy {
		return b.assign(entity.EntityID(c.Pawn), c.Index, w)
	}
	if c.Kind != command.EditApparelPolicy {
		return false
	}
	if c.Mode == command.ApparelPolicyCreate {
		return b.create(c.Index, c.Name)
	}
	i := b.find(c.Index)
	if i < 0 {
		return false
	}
	switch c.Mode {
	case command.ApparelPolicyRename:
		return b.rename(c.Index, c.Name)
	case command.ApparelPolicyDelete:
		return b.remove(c.Index)
	case command.ApparelPolicyDefault:
		return b.setDefault(c.Index, w)
	}
	p := b.Policies()[i]
	switch c.Mode {
	case command.ApparelPolicyAllow:
		f, ok := zone.Resolve(c.Name)
		if f = f.And(apparel); !ok || f == (zone.Filter{}) {
			return false
		}
		p.Allow = toggle(p.Allow, f, c.Enabled)
	case command.ApparelPolicyStuff:
		k, ok := item.ParseKind(c.Name)
		if !ok || k.StuffCategory() == 0 {
			return false
		}
		p.NoStuff = toggle(p.NoStuff, zone.Filter{}.With(k, true), !c.Enabled)
	case command.ApparelPolicyHP:
		p.Special = p.Special.Set(zone.SpecialHP, true, c.X, c.Y)
	case command.ApparelPolicyQuality:
		p.Special = p.Special.Set(zone.SpecialQuality, true, c.X, c.Y)
	case command.ApparelPolicyTainted:
		p.Special = p.Special.Set(zone.SpecialTainted, c.Enabled, 0, 0)
	default:
		return false
	}
	b.own()
	b.pols[i] = p
	return true
}

func toggle(f, set zone.Filter, on bool) zone.Filter {
	if on {
		return f.Or(set)
	}
	return f.AndNot(set)
}

func (b *Book) assign(id entity.EntityID, pid int, w Wearers) bool {
	if !w.Wearer(id) || b.find(pid) < 0 {
		return false
	}
	if pid == b.def {
		delete(b.of, id) // on the default (keeps an untouched book empty)
		return true
	}
	if b.of == nil {
		b.of = map[entity.EntityID]int{}
	}
	b.of[id] = pid
	return true
}

// own makes the book's policies its own copy before an edit.
func (b *Book) own() {
	b.views = nil
	if b.pols == nil {
		b.pols = Defaults()
	}
	if b.next < numDefaults {
		b.next = numDefaults
	}
}

func (b *Book) create(from int, name string) bool {
	p := defaults[Anything]
	if i := b.find(from); i >= 0 {
		p = b.Policies()[i]
	}
	b.own()
	p.ID, p.Name = b.next, b.unique(name, -1)
	b.pols = append(b.pols, p)
	b.next++
	return true
}

func (b *Book) rename(id int, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	b.own()
	b.pols[b.find(id)].Name = b.unique(name, id)
	return true
}

// remove deletes policy id; its holders go back to the default.
func (b *Book) remove(id int) bool {
	if id == b.def || len(b.Policies()) <= 1 {
		return false
	}
	b.own()
	b.pols = slices.Delete(b.pols, b.find(id), b.find(id)+1)
	for e, pid := range b.of {
		if pid == id {
			delete(b.of, e)
		}
	}
	return true
}

// setDefault makes policy id the one new colonists get. Everyone who was
// on the old default without a choice of their own keeps it.
func (b *Book) setDefault(id int, w Wearers) bool {
	if id == b.def {
		return false
	}
	for _, e := range w.Wearers() {
		if _, ok := b.of[e]; !ok {
			if b.of == nil {
				b.of = map[entity.EntityID]int{}
			}
			b.of[e] = b.def
		}
	}
	b.def, b.views = id, nil
	for e, pid := range b.of {
		if pid == id {
			delete(b.of, e) // on the new default
		}
	}
	return true
}

// unique is name trimmed and cut to nameMax, numbered when another
// policy than self already has it.
func (b *Book) unique(name string, self int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Outfit"
	}
	if len(name) > nameMax {
		name = name[:nameMax]
	}
	taken := func(n string) bool {
		return slices.ContainsFunc(b.Policies(), func(p Policy) bool { return p.ID != self && p.Name == n })
	}
	out := name
	for n := 2; taken(out); n++ {
		out = name + " " + strconv.Itoa(n)
	}
	return out
}

// Save is the book's saved form (nil: the defaults, nobody assigned).
type Save struct {
	Policies []Policy
	Next     int
	Default  int
	Assigned []Assigned
}

// Assigned is one colonist's chosen policy.
type Assigned struct {
	Pawn   entity.EntityID
	Policy int
}

// Export is the book's save field, assignments in id order (nil at the
// defaults, so colonies that never touch outfits hash and save as before).
func (b *Book) Export() *Save {
	if b == nil || b.pols == nil && b.def == Anything && len(b.of) == 0 {
		return nil
	}
	s := &Save{Policies: slices.Clone(b.pols), Next: b.next, Default: b.def}
	for id, p := range b.of {
		s.Assigned = append(s.Assigned, Assigned{id, p})
	}
	slices.SortFunc(s.Assigned, func(a, b Assigned) int { return cmp.Compare(a.Pawn, b.Pawn) })
	return s
}

// Import restores Export's state (nil: an older save, the defaults with
// everyone on Anything).
func (b *Book) Import(s *Save) {
	*b = Book{}
	if s == nil {
		return
	}
	if len(s.Policies) > 0 {
		b.pols, b.next = slices.Clone(s.Policies), s.Next
	}
	b.def = s.Default
	for _, a := range s.Assigned {
		if b.of == nil {
			b.of = map[entity.EntityID]int{}
		}
		b.of[a.Pawn] = a.Policy
	}
}
