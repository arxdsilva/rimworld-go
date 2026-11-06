package gear

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/combat/armor"
)

// The tab's rows as plain values (no widgets), so tests read them without
// a kit.

// Row kinds.
const (
	rowLine = iota // a summary or note line
	rowHead        // a section heading
	rowItem        // a weapon, garment or pack entry with buttons
)

// MarkUnload / DropGear / OpenInfoCard modes (command.MarkUnload's).
const (
	modePack   uint8 = 0
	modeWorn   uint8 = 1
	modeWeapon uint8 = 2
	modeSpare  uint8 = 3
)

// Row is one line of the tab.
type Row struct {
	kind  int
	Label string // the text (an item's label with quality)
	Facts string // an item's "HP 80%   1.2 kg" ("" none)
	Tip   string // the hover card ("" none)
	Dim   bool   // a note, drawn dim
	Warn  bool   // over a limit, drawn in the warning colour
	Bar   int    // a charge bar in percent (0 none)

	// An item row's target: What ("weapon", "spare", "worn", "pack") and
	// N (the n-th garment, or the pack entry ID) as MarkUnload names it.
	What    string
	Mode    uint8
	N       int
	Sprite  string // the item kind whose sprite it shows
	Tainted bool
	Forced  bool         // a garment the apparel policy leaves on
	Use     gearview.Use // what Eat does with a pack entry
	EatWhy  string       // why Eat is refused ("" allowed)
	Marked  bool         // a pack entry marked for unloading
	garment bool         // a worn garment: it has the lock
}

// Item reports r is a weapon, garment or pack entry.
func (r Row) Item() bool { return r.kind == rowItem }

// Head reports r is a section heading.
func (r Row) Head() bool { return r.kind == rowHead }

// Rows are the tab's rows for p: the summary, then Equipment, Apparel and
// the pack, then the hint.
func Rows(p game.PawnView, f Facts) []Row {
	out := Summary(p, f)
	out = append(out, equipment(f, p)...)
	out = append(out, apparel(p, f)...)
	out = append(out, pack(f, p)...)
	return append(out, Row{kind: rowLine, Dim: true, Label: f.T("Weapons and clothes lying nearby: the Equip command.")})
}

// Summary are the lines over the sections: mass against carrying
// capacity, the pack against its own limit, the comfortable range,
// overall armour, move speed (when gear changes it) and the apparel
// policy.
func Summary(p game.PawnView, f Facts) []Row {
	g := p.Gear
	var out []Row
	if g.CapacityGrams > 0 || g.GearGrams > 0 {
		out = append(out, Row{kind: rowLine, Warn: g.GearGrams > g.CapacityGrams,
			Label: lang.Tf(f, "Mass carried  %.1f / %s", float64(g.GearGrams)/1000, kg(f, g.CapacityGrams)),
			Tip:   f.T("Mass carried\nWhat is worn, wielded and packed, against what this body can carry: 75 kg, scaled by how well the hands work and by body size.")})
	}
	if p.PackLimitGrams > 0 {
		out = append(out, Row{kind: rowLine, Warn: p.PackGrams > p.PackLimitGrams,
			Label: lang.Tf(f, "Pack  %.1f / %s", float64(p.PackGrams)/1000, kg(f, p.PackLimitGrams)),
			Tip:   f.T("Pack\nWhat fits on the back. The pack has its own limit, smaller than what the body can carry.")})
	}
	out = append(out, Row{kind: rowLine,
		Label: lang.Tf(f, "Comfortable  %s to %s", f.Temp(p.ComfortMin), f.Temp(p.ComfortMax)),
		Tip:   f.T("Comfortable temperatures\nThe air temperatures this colonist is at ease in, with what they wear.")})
	out = append(out, Row{kind: rowLine, Label: f.T("Armour  ") + ArmorText(f, g.OverallArmor),
		Tip: f.T("Overall armour\nEach body part's protection from every layer over it, weighted by how often that part is hit.")})
	if g.MovePct != 0 {
		out = append(out, Row{kind: rowLine, Warn: g.MovePct < 0, Label: lang.Tf(f, "Move speed  %+d%% from gear", g.MovePct),
			Tip: f.T("Move speed\nHeavy armour slows its wearer; the penalties of several pieces add up.")})
	}
	if g.ApparelPolicyName != "" {
		out = append(out, Row{kind: rowLine, Label: f.T("Apparel policy: ") + g.ApparelPolicyName,
			Tip: f.T("Apparel policy\nWhat this colonist may put on by themselves. Locked garments stay on regardless.")})
	}
	return out
}

func head(s string) Row { return Row{kind: rowHead, Label: s} }

func note(s string) Row { return Row{kind: rowLine, Dim: true, Label: s} }

// equipment is the weapon, the spare, the shield charge, smokepops,
// grenades and the combat record.
func equipment(tr lang.Tr, p game.PawnView) []Row {
	out := []Row{head(tr.T("Equipment"))}
	if p.Weapon != "" && p.Weapon != "fists" {
		tip := WeaponTip(tr, p)
		if l := WeaponLines(tr, p); len(l) > 0 {
			tip += "\n" + strings.Join(l, "\n")
		}
		if tip == "" {
			tip = Capitalize(WeaponTitle(p))
		}
		out = append(out, Row{kind: rowItem, What: "weapon", Mode: modeWeapon, Sprite: p.Weapon,
			Label: Capitalize(WeaponTitle(p)), Facts: tr.T("in hand"), Tip: tip})
	} else {
		out = append(out, note(tr.T("No weapon")))
	}
	if p.Spare != "" {
		out = append(out, Row{kind: rowItem, What: "spare", Mode: modeSpare, Sprite: p.Spare,
			Label: Capitalize(p.Spare), Facts: tr.T("spare"), Tip: Capitalize(p.Spare) + tr.T("\nCarried as a spare, not wielded.")})
	}
	if p.ShieldPct > 0 {
		out = append(out, Row{kind: rowLine, Label: tr.T("Shield"), Bar: p.ShieldPct,
			Tip: lang.Tf(tr, "Shield belt\nCharge %d%%: it stops shots until it runs out.", p.ShieldPct)})
	}
	if p.Smokepops > 0 { // GEAR-012
		out = append(out, Row{kind: rowLine, Label: lang.Tf(tr, "Smokepop uses left: %d", p.Smokepops),
			Tip: tr.T("Smokepop pack\nEach use fills the air around the wearer with smoke that spoils aim.")})
	}
	if p.Grenades > 0 {
		out = append(out, Row{kind: rowLine, Label: lang.Tf(tr, "Grenades: %d", p.Grenades)})
	}
	if p.Kills > 0 || p.DamageDealt > 0 {
		out = append(out, note(lang.Tf(tr, "Kills %d   Damage dealt %d", p.Kills, p.DamageDealt)))
	}
	return out
}

// apparel is the worn garments in layer order and the one being put on.
func apparel(p game.PawnView, f Facts) []Row {
	out := []Row{head(f.T("Apparel"))}
	for _, i := range f.LayerOrder(p.Apparel) {
		r := Row{kind: rowItem, What: "worn", Mode: modeWorn, N: i, garment: true, Sprite: p.Apparel[i],
			Label: f.LayerTag(p.Apparel[i]) + GarmentTitle(p, i)}
		r.Forced = i < len(p.ApparelForced) && p.ApparelForced[i]
		if i < len(p.Worn) {
			w := p.Worn[i]
			r.Tainted = w.Tainted
			r.Facts = itemFacts(f, w.HP, w.MaxHP, w.Grams)
		}
		r.Tip = garmentTip(p, i, f, r.Forced)
		out = append(out, r)
	}
	if len(p.Apparel) == 0 {
		out = append(out, note(f.T("Everyday clothes only")))
	}
	if p.ChangingInto != "" {
		out = append(out, note(f.T("Changing into ")+p.ChangingInto))
	}
	return out
}

// itemFacts is "HP 80%   1.2 kg" (either part left out when unknown).
func itemFacts(tr lang.Tr, hp, maxHP, grams int) string {
	var parts []string
	if maxHP > 0 {
		parts = append(parts, lang.Tf(tr, "HP %d%%", hp*100/maxHP))
	}
	if grams > 0 {
		parts = append(parts, kg(tr, grams))
	}
	return strings.Join(parts, "   ")
}

// garmentTip is a garment row's hover card: armour, insulation, layer,
// what it covers, the move offset, what it does, the lock and its state.
func garmentTip(p game.PawnView, i int, f Facts, forced bool) string {
	lines := []string{GarmentTitle(p, i)}
	kind := p.Apparel[i]
	var w game.WornView
	if i < len(p.Worn) {
		w = p.Worn[i]
	}
	if w.MaxHP > 0 {
		lines = append(lines, lang.Tf(f, "Hit points %d / %d", w.HP, w.MaxHP))
	}
	if i < len(p.ApparelArmor) && p.ApparelArmor[i] != (armor.Set{}) {
		lines = append(lines, f.T("Armour: ")+ArmorText(f, p.ApparelArmor[i]))
	}
	if ins := f.Insulation(kind, w.Stuff, w.Quality); ins != "" {
		lines = append(lines, ins)
	}
	if tag := strings.TrimSuffix(f.LayerTag(kind), ": "); tag != "" {
		lines = append(lines, f.T("Layer: ")+tag)
	}
	if w.Cover != "" {
		lines = append(lines, f.T("Covers: ")+w.Cover)
	}
	if w.MovePct != 0 {
		lines = append(lines, lang.Tf(f, "Move speed %+d%%", w.MovePct))
	}
	if n := f.Note(kind); n != kind {
		if _, what, ok := strings.Cut(n, "("); ok {
			lines = append(lines, Capitalize(strings.TrimSuffix(strings.TrimSpace(what), ")")))
		}
	}
	switch {
	case w.Tattered:
		lines = append(lines, f.T("Tattered: the wearer minds it a lot."))
	case w.Ratty:
		lines = append(lines, f.T("Ratty: the wearer minds it."))
	}
	if w.Tainted {
		lines = append(lines, f.T("Tainted: someone died in it, and wearers feel uneasy."))
	}
	if forced {
		lines = append(lines, f.T("Locked: it stays on whatever the apparel policy says."))
	}
	return strings.Join(lines, "\n")
}

// pack is what the pawn carries in its pack.
func pack(tr lang.Tr, p game.PawnView) []Row {
	if len(p.Pack) == 0 {
		return nil
	}
	out := []Row{head(tr.T("Pack"))}
	for _, it := range p.Pack {
		label := Capitalize(it.Name)
		if it.Qty > 1 {
			label += lang.Tf(tr, " x%d", it.Qty)
		}
		out = append(out, Row{kind: rowItem, What: "pack", Mode: modePack, N: int(it.ID), Sprite: it.Name,
			Label: label, Facts: itemFacts(tr, 0, 0, it.Gear.Grams), Tip: label + tr.T("\nIn the pack."),
			Use: it.Gear.Use, EatWhy: it.Gear.Why, Marked: it.Unload})
	}
	return out
}

// Lines are the tab's text, one string per row (tests and accessibility).
func Lines(p game.PawnView, f Facts) []string {
	rows := Rows(p, f)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		l := r.Label
		if r.Facts != "" {
			l += "  " + r.Facts
		}
		if r.Tainted {
			l += "  " + f.T("tainted")
		}
		out = append(out, l)
	}
	return out
}
