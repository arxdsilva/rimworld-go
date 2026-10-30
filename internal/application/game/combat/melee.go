package combat

import (
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Melee (CBT-2). A blow picks one of the attacker's tools (weapon.Pick):
// a melee weapon's handle, blade or point, a gun's stock or barrel, a
// human's fists, head or teeth, an animal's roster attack or its second
// one. It hits by the wiki's melee hit curve (attack.HitChanceOf), the
// defender may still dodge by its own melee skill (attack.DodgeChanceOf),
// and its damage varies x0.8..1.2 (attack.MeleeDamage). Every draw comes
// from the attack's roll (the main stream, or a brawl's social stream):
// the tool when there is a choice, the hit, the dodge when the defender
// can dodge, the damage on a hit, then the armor (Wounds.Absorb). A
// humanlike with a gun fights a target beside it with the stock instead
// of shooting point-blank, as vanilla does; animals and mechanoids shoot.

// melee reports whether attacker, wielding wk (profile d) at (ax, ay),
// deals its attack on (tx, ty) as a blow: a melee weapon always, a
// humanlike's carried gun or grenade when the target is beside it.
func (s *Strikes) melee(attacker entity.EntityID, wk weapon.Kind, d weapon.Def, ax, ay, tx, ty int) bool {
	if !d.Ranged {
		return true
	}
	if weapon.Distance(ax, ay, tx, ty) > 1 || !weapon.Stock(wk) {
		return false
	}
	_, beast := s.h.Animal(attacker)
	return !beast
}

// blow resolves a melee attack of attacker on target with weapon wk (hk
// the kind its hit takes its form from: an animal's own attack; d its
// profile) and returns the tool's profile (its cooldown follows).
func (s *Strikes) blow(attacker, target entity.EntityID, wk, hk weapon.Kind, d weapon.Def, ax, ay, tx, ty int, roll func(int) int) weapon.Def {
	h := s.h
	t := s.tool(attacker, wk, hk, d, roll)
	m := t.Melee(d)
	if d.Ranged {
		m.Name = t.Name // a gun's stock is no gunshot (the trace's sound)
	}
	r := s.report(attacker, ax, ay, target, tx, ty, m, false)
	hit := roll(1000) < r.Total()
	if hit {
		if c := attack.DodgeChanceOf(h.Melee(target)); c > 0 && roll(1000) < c {
			hit = false // dodged
		}
	}
	s.learn(attacker, m, hit)
	var dh damage.Hit
	if hit {
		dh = t.Hit(d.Name, attack.MeleeDamage(t.Damage, roll))
	}
	s.land(attacker, target, ax, ay, tx, ty, wk, m, dh, hit, roll)
	return m
}

// tool is the tool attacker's blow with wk lands with: an animal's (its
// roster attack, scaled to it, or its second one), the weapon's or bare
// hands' (weapon.ToolsOf), else the weapon's own profile as its one tool
// (the mechanoids' blades).
func (s *Strikes) tool(attacker entity.EntityID, wk, hk weapon.Kind, d weapon.Def, roll func(int) int) weapon.Tool {
	if _, beast := s.h.Animal(attacker); beast {
		ts, n := weapon.NaturalTools(hk, d)
		return weapon.Pick(ts[:n], roll)
	}
	if ts := weapon.ToolsOf(wk); len(ts) > 0 {
		return weapon.Pick(ts, roll)
	}
	return weapon.Tool{Name: d.Name, Damage: d.Damage, Pen: weapon.Penetration(d, d.Damage), Cooldown: d.Cooldown,
		Form: weapon.HitOf(hk, d).Form, Chance: 1000}
}
