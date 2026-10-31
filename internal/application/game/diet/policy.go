// Package diet is what colonists, prisoners and tame animals eat (FOOD-4,
// ENG-003: RimWorld wiki numbers, original names and prose): food
// policies, food choice by optimality, the thoughts a meal gives, the
// food poisoning chances and the colony's days of food.
//
// It is a consumer sub-package of internal/application/game
// (.cursor/SOLID_GO.md §10): it never imports the game package. The
// policy book is plain state the game keeps and saves; the rest are pure
// functions of a stack, a distance and the eater.
package diet

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The default policies' ids (wiki: Assign, the food policy defaults:
// Lavish, Fine, Simple, Paste, Raw, Nothing). AnimalFeed is the tame
// animals' own default (original: raw food and animal feed, never a
// meal, the way animals are fed first). Ids are never reused.
const (
	Lavish = iota
	Fine
	Simple
	Paste
	Raw
	Nothing
	AnimalFeed
	numDefaults
)

// nameMax is the longest policy name (the editor's field).
const nameMax = 24

// Policy is one food policy: the foods it allows (kinds of the Foods
// branch of the filter tree).
type Policy struct {
	ID    int
	Name  string
	Allow zone.Filter
}

// foods and defaults are worked out once and only read afterwards (food
// choice reads them for every stack it weighs).
var (
	foods    = resolve("Foods")
	defaults = makeDefaults()
)

func resolve(name string) zone.Filter {
	f, _ := zone.Resolve(name)
	return f
}

// Foods is every kind a policy can allow: the Foods branch.
func Foods() zone.Filter { return foods }

// Defaults are the starting policies (wiki: Assign), a fresh copy: each
// allows less than the one before; none allows human or insect meat but
// the animals'.
func Defaults() []Policy { return slices.Clone(defaults) }

func makeDefaults() []Policy {
	lavish := foods.AndNot(kinds(item.HumanMeat, item.InsectMeat))
	fine := lavish.AndNot(kinds(item.LavishMeal))
	simple := fine.AndNot(kinds(item.FineMeal, item.SurvivalMeal))
	paste := simple.AndNot(kinds(item.Meal, item.Food)) // pemmican stays (wiki)
	raw := paste.AndNot(kinds(item.NutrientPasteMeal, item.Pemmican))
	feed := raw.Or(kinds(item.HumanMeat, item.InsectMeat))
	return []Policy{{Lavish, "Lavish", lavish}, {Fine, "Fine", fine}, {Simple, "Simple", simple}, {Paste, "Paste", paste},
		{Raw, "Raw", raw}, {Nothing, "Nothing", zone.Filter{}}, {AnimalFeed, "Animal feed", feed}}
}

func kinds(ks ...item.Kind) (f zone.Filter) {
	for _, k := range ks {
		f = f.With(k, true)
	}
	return f
}

// Eaters tells the book who may hold a policy: a colonist or prisoner
// (person), or a tame animal.
type Eaters interface {
	// Eater reports whether id may hold a food policy, and if it is a
	// tame animal.
	Eater(id entity.EntityID) (animal, ok bool)
}

// Book is the colony's food policies and who holds which. The zero Book
// is the defaults with nobody assigned (people hold the first policy,
// tame animals Animal feed).
type Book struct {
	pols []Policy                // nil: Defaults()
	next int                     // the next new policy's id (0: numDefaults)
	of   map[entity.EntityID]int // assigned policy ids
	// named caches Named until the next edit (never saved).
	named []Named
}

// Named is a policy as the read model shows it: its allowed foods as the
// filter tree names them.
type Named struct {
	ID      int
	Name    string
	Allowed []string
}

// Named lists the policies with their allowed foods' names (shared: read
// only; worked out once per edit, the views read it every frame).
func (b *Book) Named() []Named {
	if b.named == nil {
		for _, p := range b.Policies() {
			b.named = append(b.named, Named{p.ID, p.Name, p.Allow.Names()})
		}
	}
	return b.named
}

// Policies lists the policies in order, the first the default (shared:
// read only).
func (b *Book) Policies() []Policy {
	if b.pols == nil {
		return defaults
	}
	return b.pols
}

func (b *Book) find(id int) (Policy, int) {
	for i, p := range b.Policies() {
		if p.ID == id {
			return p, i
		}
	}
	return Policy{}, -1
}

// Of is eater id's policy: the one assigned, else the default for a
// person or a tame animal (Animal feed while it exists).
func (b *Book) Of(id entity.EntityID, animal bool) Policy {
	if pid, ok := b.of[id]; ok {
		if p, i := b.find(pid); i >= 0 {
			return p
		}
	}
	if animal {
		if p, i := b.find(AnimalFeed); i >= 0 {
			return p
		}
	}
	return b.Policies()[0]
}

// Menu is what eater id's policy lets it eat, worked out once for a
// search over many stacks.
func (b *Book) Menu(id entity.EntityID, animal bool) Menu {
	return Menu{allow: b.Of(id, animal).Allow, foods: foods}
}

// Menu is a policy ready for checking stacks.
type Menu struct{ allow, foods zone.Filter }

// Has reports the policy allows kind k; kinds outside the Foods branch
// (corpses) are not a policy's to forbid.
func (m Menu) Has(k item.Kind) bool { return !m.foods.Has(k) || m.allow.Has(k) }

// Apply runs a SetFoodPolicy or EditFoodPolicy command; it reports
// whether anything changed. Unknown policies, eaters that may hold none,
// deleting the default and empty names change nothing.
func (b *Book) Apply(c command.Command, e Eaters) bool {
	if c.Kind == command.SetFoodPolicy {
		id := entity.EntityID(c.Pawn)
		if _, ok := e.Eater(id); !ok || b.lookup(c.Index) < 0 {
			return false
		}
		if b.of == nil {
			b.of = map[entity.EntityID]int{}
		}
		b.of[id] = c.Index
		return true
	}
	if c.Kind != command.EditFoodPolicy {
		return false
	}
	switch c.Mode {
	case command.FoodPolicyCreate:
		return b.create(c.Index, c.Name)
	case command.FoodPolicyAllow:
		return b.allow(c.Index, c.Name, c.Enabled)
	case command.FoodPolicyRename:
		return b.rename(c.Index, c.Name)
	case command.FoodPolicyDelete:
		return b.remove(c.Index)
	}
	return false
}

func (b *Book) lookup(id int) int { _, i := b.find(id); return i }

// own makes the book's policies its own copy before an edit.
func (b *Book) own() {
	b.named = nil
	if b.pols == nil {
		b.pols = Defaults()
	}
	if b.next < numDefaults {
		b.next = numDefaults
	}
}

func (b *Book) create(from int, name string) bool {
	allow := Defaults()[Lavish].Allow
	if p, i := b.find(from); i >= 0 {
		allow = p.Allow
	}
	b.own()
	b.pols = append(b.pols, Policy{ID: b.next, Name: b.unique(name, -1), Allow: allow})
	b.next++
	return true
}

func (b *Book) allow(id int, name string, on bool) bool {
	f, ok := zone.Resolve(name)
	f = f.And(foods)
	if !ok || f == (zone.Filter{}) || b.lookup(id) < 0 {
		return false
	}
	b.own()
	p := &b.pols[b.lookup(id)]
	if on {
		p.Allow = p.Allow.Or(f)
	} else {
		p.Allow = p.Allow.AndNot(f)
	}
	return true
}

func (b *Book) rename(id int, name string) bool {
	if i := b.lookup(id); i < 0 || strings.TrimSpace(name) == "" {
		return false
	}
	b.own()
	i := b.lookup(id)
	b.pols[i].Name = b.unique(name, id)
	return true
}

func (b *Book) remove(id int) bool {
	i := b.lookup(id)
	if i <= 0 { // unknown, or the default
		return false
	}
	b.own()
	b.pols = append(b.pols[:i:i], b.pols[i+1:]...)
	for e, pid := range b.of {
		if pid == id {
			delete(b.of, e) // back to the default
		}
	}
	return true
}

// unique is name trimmed and cut to nameMax, numbered when another
// policy than self already has it (the editor cycles policies by name).
func (b *Book) unique(name string, self int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Policy"
	}
	if len(name) > nameMax {
		name = name[:nameMax]
	}
	taken := func(n string) bool {
		for _, p := range b.Policies() {
			if p.ID != self && p.Name == n {
				return true
			}
		}
		return false
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
	Assigned []Assigned
}

// Assigned is one eater's chosen policy.
type Assigned struct {
	Eater  entity.EntityID
	Policy int
}

// Export is the book's save field, assignments in id order (nil at the
// defaults, so older colonies hash and save as before).
func (b *Book) Export() *Save {
	if b.pols == nil && len(b.of) == 0 {
		return nil
	}
	s := &Save{Policies: append([]Policy(nil), b.pols...), Next: b.next}
	for id, p := range b.of {
		s.Assigned = append(s.Assigned, Assigned{id, p})
	}
	slices.SortFunc(s.Assigned, func(a, b Assigned) int { return cmp.Compare(a.Eater, b.Eater) })
	return s
}

// Import restores Export's state (nil: an older save, the defaults).
func (b *Book) Import(s *Save) {
	*b = Book{}
	if s == nil {
		return
	}
	if len(s.Policies) > 0 {
		b.pols, b.next = append([]Policy(nil), s.Policies...), s.Next
	}
	for _, a := range s.Assigned {
		if b.of == nil {
			b.of = map[entity.EntityID]int{}
		}
		b.of[a.Eater] = a.Policy
	}
}
