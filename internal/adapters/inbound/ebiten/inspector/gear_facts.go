package inspector

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Quality, stuff, condition, weapon and armor lines from the read models
// (PRD-2, DEF-2, UI checklist 12.3).

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

// WeaponLines are the weapon's numbers for the Gear tab; accuracy is at
// touch, short, medium and long range.
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

// WornLine is one garment: "Steel plate (good, tattered): sharp 40% ..."
// with its armor when it has any.
func WornLine(p game.PawnView, i int) string {
	var name string
	if i < len(p.Worn) {
		w := p.Worn[i]
		name = QualifiedTitle(Capitalize(w.Kind), w.Stuff, w.Quality)
		var tags []string
		if w.Tattered {
			tags = append(tags, "tattered")
		} else if w.Ratty {
			tags = append(tags, "ratty")
		}
		if w.Tainted {
			tags = append(tags, "worn by a dead pawn")
		}
		if len(tags) > 0 {
			name += " [" + strings.Join(tags, ", ") + "]"
		}
	} else if i < len(p.Apparel) {
		name = Capitalize(p.Apparel[i])
	}
	if i < len(p.ApparelArmor) {
		a := p.ApparelArmor[i]
		return fmt.Sprintf("%s: sharp %d%%  blunt %d%%  heat %d%%", name, int(a.Sharp)/10, int(a.Blunt)/10, int(a.Heat)/10)
	}
	return name
}

// GearStatLines are the Gear tab's extra lines: the weapon's numbers and
// each worn garment with its condition and armor.
func GearStatLines(p game.PawnView) []string {
	out := WeaponLines(p)
	n := max(len(p.Worn), len(p.ApparelArmor))
	for i := 0; i < n; i++ {
		out = append(out, WornLine(p, i))
	}
	return append(out, CombatExtraLines(p)...)
}

// InspirationLine is the pawn status line for an active inspiration
// ("inspired creativity" becomes "Inspired: creativity").
func InspirationLine(insp string) string {
	if insp == "" {
		return ""
	}
	return "Inspired: " + strings.TrimPrefix(insp, "inspired ")
}
