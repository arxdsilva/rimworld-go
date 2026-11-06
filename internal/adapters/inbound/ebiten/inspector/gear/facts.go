package gear

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/clock"
)

// Quality, stuff, condition, weapon and armour text from the read models
// (PRD-2, DEF-2, CBT-5, UI checklist 12.3). The item pane uses
// QualifiedTitle and ItemFactLines too.

// Capitalize upper-cases the first byte of s.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// QualifiedTitle is the stack title with its material and quality:
// "Steel longsword (good)". The material prefixes the kind when the title
// does not already name it.
func QualifiedTitle(title, stuff, quality string) string {
	if stuff != "" && title != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(stuff)) {
		title = Capitalize(stuff) + " " + strings.ToLower(title[:1]) + title[1:]
	}
	if quality != "" {
		title += " (" + quality + ")"
	}
	return title
}

// ItemFactLines are the condition lines for a stack: hit points and
// tainted apparel.
func ItemFactLines(tr lang.Tr, it game.ItemView) []string {
	var out []string
	if it.MaxHP > 0 {
		out = append(out, lang.Tf(tr, "Hit points %d / %d (%d%%)", it.HP, it.MaxHP, it.HP*100/it.MaxHP))
	}
	if it.Tainted {
		out = append(out, tr.T("Worn by a dead pawn: wearers feel uneasy"))
	}
	return out
}

func secs(ticks int) float64 { return float64(ticks) / float64(clock.TicksPerSecond) }

// WeaponTitle is the wielded weapon with its stuff and quality
// ("Steel longsword (good)"); WeaponQuality is quality.Level+1 (0: none).
func WeaponTitle(p game.PawnView) string {
	q := ""
	if p.WeaponQuality > 0 && quality.Level(p.WeaponQuality-1).Valid() {
		q = quality.Level(p.WeaponQuality - 1).String()
	}
	return QualifiedTitle(p.Weapon, p.WeaponStuff, q)
}

// WeaponTip is the weapon row's tooltip: its damage and cooldown, and for
// a melee weapon what that comes to each second, then its numbers.
func WeaponTip(tr lang.Tr, p game.PawnView) string {
	w := p.WeaponStats
	if w.Name == "" {
		return ""
	}
	title := Capitalize(WeaponTitle(p))
	var body string
	if w.Melee {
		body = lang.Tf(tr, "Melee. Damage %d per hit, cooldown %.1fs.", w.Damage, secs(w.Cooldown))
		if cycle := secs(w.Warmup + w.Cooldown); cycle > 0 {
			body += lang.Tf(tr, " About %.1f damage per second when every blow lands.", float64(w.Damage)/cycle)
		}
	} else {
		body = lang.Tf(tr, "Ranged. Damage %d per shot, warmup %.1fs, cooldown %.1fs, range %d.",
			w.Damage, secs(w.Warmup), secs(w.Cooldown), w.Range)
	}
	return title + "\n" + body
}

// WeaponLines are the weapon's numbers; accuracy is at touch, short,
// medium and long range.
func WeaponLines(tr lang.Tr, p game.PawnView) []string {
	w := p.WeaponStats
	if w.Name == "" {
		return nil
	}
	out := []string{lang.Tf(tr, "Damage %d   Armor penetration %d%%", w.Damage, w.ArmorPen/10)}
	if w.Melee {
		return append(out, lang.Tf(tr, "Cooldown %.1fs", secs(w.Cooldown)))
	}
	out = append(out,
		lang.Tf(tr, "Warmup %.1fs   Cooldown %.1fs   Burst %d   Range %d", secs(w.Warmup), secs(w.Cooldown), w.Burst, w.Range),
		lang.Tf(tr, "Accuracy touch %d%%  short %d%%  medium %d%%  long %d%%",
			w.Accuracy[0]/10, w.Accuracy[1]/10, w.Accuracy[2]/10, w.Accuracy[3]/10))
	if w.Blast != "" {
		out = append(out, lang.Tf(tr, "Blast: %s, radius %.1f", w.Blast, float64(w.BlastRadius)/10))
	}
	return out
}

// GarmentTitle is garment i with its stuff and quality ("Steel flak vest
// (good)").
func GarmentTitle(p game.PawnView, i int) string {
	if i < len(p.Worn) {
		w := p.Worn[i]
		return QualifiedTitle(Capitalize(w.Kind), w.Stuff, w.Quality)
	}
	if i < len(p.Apparel) {
		return Capitalize(p.Apparel[i])
	}
	return ""
}

// ArmorText is a rating set as "sharp 40%  blunt 30%  heat 5%".
func ArmorText(tr lang.Tr, a armor.Set) string {
	return lang.Tf(tr, "sharp %d%%  blunt %d%%  heat %d%%", int(a.Sharp)/10, int(a.Blunt)/10, int(a.Heat)/10)
}

// WornLine is one garment: "Steel flak vest (good) [tattered]: sharp 40%
// ..." with its armour when the read model has it.
func WornLine(tr lang.Tr, p game.PawnView, i int) string {
	name := GarmentTitle(p, i)
	if i < len(p.Worn) {
		var tags []string
		if w := p.Worn[i]; w.Tattered {
			tags = append(tags, tr.T("tattered"))
		} else if w.Ratty {
			tags = append(tags, tr.T("ratty"))
		}
		if p.Worn[i].Tainted {
			tags = append(tags, tr.T("worn by a dead pawn"))
		}
		if len(tags) > 0 {
			name += " [" + strings.Join(tags, ", ") + "]"
		}
	}
	if i < len(p.ApparelArmor) {
		return name + ": " + ArmorText(tr, p.ApparelArmor[i])
	}
	return name
}

// StatLines is StatLinesIn in English.
func StatLines(p game.PawnView) []string { return StatLinesIn(lang.English{}, p) }

// StatLinesIn are the weapon's numbers, each worn garment with its condition
// and armour, and the combat lines: the tab's text in plain lines.
func StatLinesIn(tr lang.Tr, p game.PawnView) []string {
	out := WeaponLines(tr, p)
	n := max(len(p.Worn), len(p.ApparelArmor))
	for i := 0; i < n; i++ {
		out = append(out, WornLine(tr, p, i))
	}
	return append(out, CombatExtraLines(tr, p)...)
}

// CombatExtraLines are the spare weapon and the combat record.
func CombatExtraLines(tr lang.Tr, p game.PawnView) []string {
	var out []string
	if p.Spare != "" {
		out = append(out, tr.T("Spare weapon: ")+p.Spare)
	}
	if p.Kills > 0 || p.DamageDealt > 0 {
		out = append(out, lang.Tf(tr, "Kills %d   Damage dealt %d", p.Kills, p.DamageDealt))
	}
	return out
}

// kg is grams as "1.2 kg".
func kg(tr lang.Tr, g int) string { return lang.Tf(tr, "%.1f kg", float64(g)/1000) }
