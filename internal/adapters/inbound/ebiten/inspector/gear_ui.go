package inspector

import (
	"fmt"
	"reflect"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/entity"
)

// The Gear tab's combat lines (CBT-5 UI): the weapon with its quality and
// stuff, the weapon tooltip, the spare weapon, the shield belt charge and
// the pawn's combat record. CBT-3 fills PawnView.Spare and ShieldPct
// beside the quality and stuff, so they are read by name and absent
// fields read as nothing.

// shieldBarH is the shield belt charge bar's height.
const shieldBarH = 13

// WeaponTitle is the wielded weapon with its stuff and quality
// ("Steel longsword (good)"); WeaponQuality is quality.Level+1 (0: none).
func WeaponTitle(p game.PawnView) string {
	q := ""
	if p.WeaponQuality > 0 && quality.Level(p.WeaponQuality-1).Valid() {
		q = quality.Level(p.WeaponQuality - 1).String()
	}
	return QualifiedTitle(p.Weapon, p.WeaponStuff, q)
}

// WeaponTip is the weapon line's tooltip: its damage and cooldown, and
// for a melee weapon what that comes to each second (UI checklist 12.3).
func WeaponTip(p game.PawnView) string {
	w := p.WeaponStats
	if w.Name == "" {
		return ""
	}
	title := Capitalize(WeaponTitle(p))
	if w.Melee {
		cycle := secs(w.Warmup + w.Cooldown)
		body := fmt.Sprintf("Melee. Damage %d per hit, cooldown %.1fs.", w.Damage, secs(w.Cooldown))
		if cycle > 0 {
			body += fmt.Sprintf(" About %.1f damage per second when every blow lands.", float64(w.Damage)/cycle)
		}
		return title + "\n" + body
	}
	return title + "\n" + fmt.Sprintf("Ranged. Damage %d per shot, warmup %.1fs, cooldown %.1fs, range %d.",
		w.Damage, secs(w.Warmup), secs(w.Cooldown), w.Range)
}

// optField is the exported field name of a PawnView, when it exists.
func optField(p game.PawnView, name string) (reflect.Value, bool) {
	f := reflect.ValueOf(p).FieldByName(name)
	return f, f.IsValid()
}

// ShieldCharge is the shield belt's charge in percent; ok is false when
// the pawn wears none (or the read model has no such field yet).
func ShieldCharge(p game.PawnView) (pct int, ok bool) {
	f, has := optField(p, "ShieldPct")
	if !has || !f.CanInt() || f.Int() <= 0 {
		return 0, false
	}
	return int(f.Int()), true
}

// SpareName is the spare weapon the pawn carries ("" when none). The
// field may be a name, or a slot holding an item kind and a quantity.
func SpareName(p game.PawnView) string {
	f, ok := optField(p, "Spare")
	if !ok {
		return ""
	}
	switch f.Kind() {
	case reflect.String:
		return f.String()
	case reflect.Struct:
		qty, k := f.FieldByName("Qty"), f.FieldByName("Kind")
		if !qty.IsValid() || !k.IsValid() || !qty.CanInt() || qty.Int() <= 0 {
			return ""
		}
		name := fmt.Sprint(k.Interface())
		if qty.Int() > 1 {
			name += fmt.Sprintf(" x%d", qty.Int())
		}
		return name
	}
	return ""
}

// CombatExtraLines are the Gear tab's spare weapon and combat record
// lines.
func CombatExtraLines(p game.PawnView) []string {
	var out []string
	if s := SpareName(p); s != "" {
		out = append(out, "Spare weapon: "+s)
	}
	if p.Kills > 0 || p.DamageDealt > 0 {
		out = append(out, fmt.Sprintf("Kills %d   Damage dealt %d", p.Kills, p.DamageDealt))
	}
	return out
}

// gearTabBody declares the Gear tab of colonist id inside r: the lists
// (drawn through the canvas queue), the weapon tooltip and the shield
// charge bar.
func (a Tabs) gearTabBody(id entity.EntityID, r gfx.Rect) {
	p, ok := a.pawnView(id)
	if !ok {
		return
	}
	lines := GearStatLines(p)
	rows := len(GearRows(p)) - 1
	g := GearLayoutFor(rows)
	step := gfx.LineStep(gfx.Small)
	if tip := WeaponTip(p); tip != "" {
		a.kit().Tip("inspector.gear.weapon", gfx.Rect{X: r.X + 12, Y: r.Y + g.WeaponVal, W: r.W - 12, H: step}, tip)
	}
	pct, shield := ShieldCharge(p)
	by := r.Y + g.Hint + step + 6 + float32(len(lines))*step + 4
	a.kit().Queue(func(dst *ebiten.Image) {
		drawGear(dst, p, r, a.shell.Temp)
		y := float64(r.Y) + float64(g.Hint) + float64(step) + 6
		for _, l := range lines {
			if float32(y)+step > r.Bottom() {
				break
			}
			gfx.Text(dst, gfx.Ellipsize(l, gfx.Small, float64(r.W)), float64(r.X), y, gfx.Small)
			y += float64(step)
		}
		if shield && by+shieldBarH <= r.Bottom() {
			drawNeedBar(dst, int(r.X), int(by), int(r.W), shieldBarH, pct, "shield")
		}
	})
}
