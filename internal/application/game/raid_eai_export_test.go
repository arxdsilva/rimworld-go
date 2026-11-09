package game

import "fmt"

// RaidIndexMismatch checks, from every raider's tile, the raiders' indexed
// look-ups (EAI-021, EAI-023) against scans of the whole lists: the nearest
// colonist, a thief's loot and a burner's mark. "" when they all agree.
func RaidIndexMismatch(g *Game) string {
	m := raidMischief{g: g, wood: &woodIndex{}}
	for _, id := range g.raiderIDs {
		p, ok := g.positions.Get(id)
		if !ok {
			continue
		}
		a, ap, aok := nearestColonistRings(g, p)
		b, bp, bok := nearestColonistScan(g, p)
		if a != b || ap != bp || aok != bok {
			return fmt.Sprintf("colonist from %+v: rings %d, scan %d", p, a, b)
		}
		for _, r := range []int{8, 30, 90} {
			if la, lx, ly, lok := m.Loot(p.X, p.Y, r); fmt.Sprint(la, lx, ly, lok) != fmt.Sprint(lootScan(g, p.X, p.Y, r)) {
				return fmt.Sprintf("loot from %+v r %d: grid %d", p, r, la)
			}
			if fx, fy, fok := m.Flammable(p.X, p.Y, r); fmt.Sprint(fx, fy, fok) != fmt.Sprint(flammableScan(g, p.X, p.Y, r)) {
				return fmt.Sprintf("wood from %+v r %d: grid (%d, %d)", p, r, fx, fy)
			}
		}
	}
	return ""
}
