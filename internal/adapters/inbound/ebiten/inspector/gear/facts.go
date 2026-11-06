package gear

import (
	"fmt"
	"strings"

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
func ItemFactLines(it game.ItemView) []string {
	var out []string
	if it.MaxHP > 0 {
		out = append(out, fmt.Sprintf("Hit points %d / %d (%d%%)", it.HP, it.MaxHP, it.HP*100/it.MaxHP))
	}
	if it.Tainted {
		out = append(out, "Worn by a dead pawn: wearers feel uneasy")
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
func WeaponTip(p game.PawnView) string {
	w := p.WeaponStats
	if w.Name == "" {
		return ""
	}
	title := Capitalize(WeaponTitle(p))
	var body string
	if w.Melee {
		body = fmt.Sprintf("Melee. Damage %d per hit, cooldown %.1fs.", w.Damage, secs(w.Cooldown))
		if cycle := secs(w.Warmup + w.Cooldown); cycle > 0 {
			body += fmt.Sprintf(" About %.1f damage per second when every blow lands.", float64(w.Damage)/cycle)
		}
	} else {
		body = fmt.Sprintf("Ranged. Damage %d per shot, warmup %.1fs, cooldown %.1fs, range %d.",
			w.Damage, secs(w.Warmup), secs(w.Cooldown), w.Range)
	}
	return title + "\n" + body
}

// WeaponLines are the weapon's numbers; accuracy is at touch, short,
// medium and long range.
func WeaponLines(p game.PawnView) []string {
	w := p.WeaponStats
	if w.Name == "" {
		return nil
	}
	out := []string{fmt.Sprintf("Damage %d   Armor penetration %d%%", w.Damage, w.ArmorPen/10)}
	if w.Melee {
		return append(out, fmt.Sprintf("Cooldown %.1fs", secs(w.Cooldown)))
	}
	out = append(out,
		fmt.Sprintf("Warmup %.1fs   Cooldown %.1fs   Burst %d   Range %d", secs(w.Warmup), secs(w.Cooldown), w.Burst, w.Range),
		fmt.Sprintf("Accuracy touch %d%%  short %d%%  medium %d%%  long %d%%",
			w.Accuracy[0]/10, w.Accuracy[1]/10, w.Accuracy[2]/10, w.Accuracy[3]/10))
	if w.Blast != "" {
		out = append(out, fmt.Sprintf("Blast: %s, radius %.1f", w.Blast, float64(w.BlastRadius)/10))
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
func ArmorText(a armor.Set) string {
	return fmt.Sprintf("sharp %d%%  blunt %d%%  heat %d%%", int(a.Sharp)/10, int(a.Blunt)/10, int(a.Heat)/10)
}

// WornLine is one garment: "Steel flak vest (good) [tattered]: sharp 40%
// ..." with its armour when the read model has it.
func WornLine(p game.PawnView, i int) string {
	name := GarmentTitle(p, i)
	if i < len(p.Worn) {
		var tags []string
		if w := p.Worn[i]; w.Tattered {
			tags = append(tags, "tattered")
		} else if w.Ratty {
			tags = append(tags, "ratty")
		}
		if p.Worn[i].Tainted {
			tags = append(tags, "worn by a dead pawn")
		}
		if len(tags) > 0 {
			name += " [" + strings.Join(tags, ", ") + "]"
		}
	}
	if i < len(p.ApparelArmor) {
		return name + ": " + ArmorText(p.ApparelArmor[i])
	}
	return name
}

// StatLines are the weapon's numbers, each worn garment with its condition
// and armour, and the combat lines: the tab's text in plain lines.
func StatLines(p game.PawnView) []string {
	out := WeaponLines(p)
	n := max(len(p.Worn), len(p.ApparelArmor))
	for i := 0; i < n; i++ {
		out = append(out, WornLine(p, i))
	}
	return append(out, CombatExtraLines(p)...)
}

// CombatExtraLines are the spare weapon and the combat record.
func CombatExtraLines(p game.PawnView) []string {
	var out []string
	if p.Spare != "" {
		out = append(out, "Spare weapon: "+p.Spare)
	}
	if p.Kills > 0 || p.DamageDealt > 0 {
		out = append(out, fmt.Sprintf("Kills %d   Damage dealt %d", p.Kills, p.DamageDealt))
	}
	return out
}

// kg is grams as "1.2 kg".
func kg(g int) string { return fmt.Sprintf("%.1f kg", float64(g)/1000) }
