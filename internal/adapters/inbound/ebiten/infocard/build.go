package infocard

import (
	"fmt"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// Row is one fact. A row with no Value is a section heading.
type Row struct{ Label, Value string }

// Card is what the window shows.
type Card struct {
	Title, Sub, Desc string
	Rows             []Row
}

// The modes of a pawn's gear row (command.DropGear's).
const (
	ModePack    = 0 // Index is the pack entry's ID
	ModeApparel = 1 // Index is the worn slot
	ModeWeapon  = 2
	ModeSpare   = 3
)

// source is the thing asked about, as far as the read models know it.
type source struct {
	name, stuff, holder string
	quality             uint8 // stored: 0 none
	hp, maxHP, qty      int
	grams               int // the stack's mass when the model gives it
	fresh               float64
	armor               *armor.Set
	wpn                 *weapon.View
	found               bool
}

// Build is the card for q, from the read models r.
func Build(r Reader, q ports.OpenInfoCard) Card {
	s := resolve(r, q)
	if !s.found {
		return Card{Title: r.T("Item"), Desc: r.T("This item is no longer there.")}
	}
	k, ok := item.ParseKind(s.name)
	c := Card{Title: upper(s.name), Sub: subtitle(r, k, ok, s), Desc: describe(r, k, ok)}
	if s.qty > 1 {
		c.add(r.T("Count"), fmt.Sprint(s.qty))
	}
	c.facts(r, s, k, ok)
	if ok {
		c.food(r, s, k)
		c.apparel(r, s, k)
	}
	c.weapon(r, s, k, ok)
	return c
}

func (c *Card) add(label, value string) { c.Rows = append(c.Rows, Row{label, value}) }

// facts are the numbers every item has: mass, worth, quality, condition.
func (c *Card) facts(tr lang.Tr, s source, k item.Kind, ok bool) {
	if ok {
		g := k.MassGrams()
		c.add(tr.T("Mass"), mass(tr, g))
		if s.qty > 1 {
			c.add(tr.T("Stack mass"), mass(tr, g*s.qty))
		}
		if v := k.Value(); v > 0 {
			c.add(tr.T("Value"), lang.Tf(tr, "%d silver", v))
		}
		if n := k.StackLimit(); n > 1 {
			c.add(tr.T("Stack limit"), fmt.Sprint(n))
		}
	} else if s.grams > 0 {
		c.add(tr.T("Mass"), mass(tr, s.grams))
	}
	if l, has := quality.FromStored(s.quality); has {
		c.add(tr.T("Quality"), l.String())
	}
	if s.maxHP > 0 {
		c.add(tr.T("Hit points"), lang.Tf(tr, "%d / %d (%d%%)", s.hp, s.maxHP, s.hp*100/s.maxHP))
	}
	if s.stuff != "" {
		c.add(tr.T("Made of"), s.stuff)
	}
}

// food adds the nutrition and the shelf life.
func (c *Card) food(tr lang.Tr, s source, k item.Kind) {
	if n := k.Nutrition(); n > 0 {
		c.add(tr.T("Nutrition"), fmt.Sprint(n))
		if k.Raw() {
			c.add(tr.T("Eaten"), tr.T("raw: it sours the mood"))
		}
	}
	if s.fresh > 0 {
		c.add(tr.T("Keeps for"), lang.Tf(tr, "%.1f days", s.fresh))
	}
	if k.IsDrug() {
		c.add(tr.T("Kind"), tr.T("drug"))
	}
}

// apparel adds what a garment shields, covers and costs in speed.
func (c *Card) apparel(tr lang.Tr, s source, k item.Kind) {
	st, _ := item.ParseKind(s.stuff)
	l, ok := armor.Piece(k, st, s.quality)
	if !ok {
		return
	}
	set := l.Set
	if s.armor != nil {
		set = *s.armor
	}
	c.add(tr.T("Armour"), "")
	c.add(tr.T("Sharp"), pct(int(set.Sharp)))
	c.add(tr.T("Blunt"), pct(int(set.Blunt)))
	c.add(tr.T("Heat"), pct(int(set.Heat)))
	c.add(tr.T("Covers"), l.Covers.Names())
	c.add(tr.T("Layers"), plies(tr, l.Plies))
	cold, heat := item.Insulation(k, st, s.quality)
	c.add(tr.T("Insulation"), lang.Tf(tr, "cold %s, heat %s", tenths(tr, cold), tenths(tr, heat)))
	if m := armor.SpeedPct(armor.MoveOffset(k)); m != 0 {
		c.add(tr.T("Move speed"), fmt.Sprintf("%+d%%", m))
	}
}

// weapon adds the weapon's numbers (the model's when it is the wielded one).
func (c *Card) weapon(tr lang.Tr, s source, k item.Kind, ok bool) {
	w := s.wpn
	if w == nil && (!ok || k.IsWeapon() || k.IsContractWeapon()) {
		if v, has := weaponByName(s.name); has {
			w = &v
		}
	}
	if w == nil {
		return
	}
	c.add(tr.T("Weapon"), "")
	c.add(tr.T("Damage"), fmt.Sprint(w.Damage))
	c.add(tr.T("Armour penetration"), pct(w.ArmorPen))
	if w.Melee {
		c.add(tr.T("Reach"), tr.T("adjacent"))
	} else {
		c.add(tr.T("Range"), lang.Tf(tr, "%d tiles", w.Range))
		c.add(tr.T("Burst"), fmt.Sprint(w.Burst))
		c.add(tr.T("Aim time"), seconds(tr, w.Warmup))
		c.add(tr.T("Accuracy"), lang.Tf(tr, "%s / %s / %s / %s at 3 / 12 / 25 / 40 tiles",
			pct(w.Accuracy[0]), pct(w.Accuracy[1]), pct(w.Accuracy[2]), pct(w.Accuracy[3])))
	}
	c.add(tr.T("Cooldown"), seconds(tr, w.Cooldown))
}

// weaponByName finds the weapon profile named name.
func weaponByName(name string) (weapon.View, bool) {
	for i := 0; i < 256; i++ {
		if k := weapon.Kind(i); weapon.DefOf(k).Name == name {
			return weapon.Stats(k), true
		}
	}
	return weapon.View{}, false
}

// resolve finds what q points at in the read models.
func resolve(r Reader, q ports.OpenInfoCard) source {
	switch {
	case q.Pawn != 0:
		return ofPawn(r, q)
	case q.ID != 0:
		for _, it := range r.World().Items {
			if it.ID == q.ID {
				return source{name: it.Kind, qty: it.Qty, fresh: it.FreshDays, found: true}
			}
		}
		return source{}
	}
	return source{name: q.Kind, fresh: -1, found: q.Kind != ""}
}

// ofPawn is the pawn's worn garment, weapon, spare or pack entry.
func ofPawn(r Reader, q ports.OpenInfoCard) source {
	var p *game.PawnView
	ps := r.Pawns()
	for i := range ps {
		if ps[i].ID == q.Pawn {
			p = &ps[i]
		}
	}
	if p == nil {
		return source{}
	}
	s := source{holder: p.Name, fresh: -1}
	switch q.Mode {
	case ModeApparel:
		if q.Index < 0 || q.Index >= len(p.Apparel) {
			return source{}
		}
		s.name, s.found = p.Apparel[q.Index], true
		if q.Index < len(p.Worn) {
			w := p.Worn[q.Index]
			s.stuff, s.hp, s.maxHP = w.Stuff, w.HP, w.MaxHP
			s.quality = qualityByLabel(w.Quality)
		}
		if q.Index < len(p.ApparelArmor) {
			a := p.ApparelArmor[q.Index]
			s.armor = &a
		}
	case ModeWeapon:
		s.name, s.stuff, s.quality, s.found = p.Weapon, p.WeaponStuff, p.WeaponQuality, p.Weapon != ""
		w := p.WeaponStats
		s.wpn = &w
	case ModeSpare:
		s.name, s.found = p.Spare, p.Spare != ""
	case ModePack:
		for _, e := range p.Pack {
			if int(e.ID) == q.Index {
				s.name, s.qty, s.grams, s.found = e.Name, e.Qty, e.Gear.Grams, true
			}
		}
	}
	return s
}

// qualityByLabel is the stored quality of a level's label (0: none).
func qualityByLabel(l string) uint8 {
	for _, lv := range quality.All() {
		if lv.String() == l {
			return quality.Stored(lv)
		}
	}
	return 0
}

func subtitle(tr lang.Tr, k item.Kind, ok bool, s source) string {
	out := tr.T("Item")
	if ok {
		out = k.Category()
	}
	if s.holder != "" {
		out += lang.Tf(tr, ", carried by %s", s.holder)
	}
	return out
}

func upper(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func mass(tr lang.Tr, g int) string {
	if g < 1000 {
		return lang.Tf(tr, "%d g", g)
	}
	return lang.Tf(tr, "%.2f kg", float64(g)/1000)
}

// pct words a permille rating as a percentage.
func pct(p int) string {
	if p%10 == 0 {
		return fmt.Sprintf("%d%%", p/10)
	}
	return fmt.Sprintf("%.1f%%", float64(p)/10)
}

func tenths(tr lang.Tr, t int) string { return lang.Tf(tr, "%.1f C", float64(t)/10) }

func seconds(tr lang.Tr, ticks int) string { return lang.Tf(tr, "%.1f s", float64(ticks)/60) }

// plies names the layers a garment takes, innermost first.
func plies(tr lang.Tr, p uint8) string {
	names := []string{tr.T("skin"), tr.T("middle"), tr.T("outer"), tr.T("belt"), tr.T("headgear"), tr.T("eyes")}
	var out []string
	for i, n := range names {
		if p&(1<<uint(i)) != 0 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return tr.T("none")
	}
	return strings.Join(out, ", ")
}
