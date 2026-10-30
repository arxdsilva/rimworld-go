package raid

import (
	"fmt"

	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
)

// Pods are hostile drop pods (DEF-4). A pod raid plans one pod per raider
// when it is sent: on the colony (around the colonist nearest home), near
// one map edge, or scattered over the map, never under thick rock (wiki:
// Raid, Drop pod). Each falls for raiddef.PodFallTicks (staggered a
// little) and lands: it breaks the roof it comes through, which falls on
// whoever is beneath, leaves its shell as debris, and its raider steps
// out. A landed pod is kept on its group for an hour for the dust and
// sounds, then forgotten.
type Pods struct {
	h  Host
	a  Arrive
	ch Chronicle
}

// Pod is one drop pod of a group (the save's RaidGroups). Fields are gob
// names: never rename.
type Pod struct {
	X, Y   int
	Land   uint64 // landing tick
	Kit    item.Kind
	Landed bool
}

// PodView is a pod for the read model.
type PodView struct {
	X, Y   int
	Land   uint64
	Landed bool
}

const (
	podKeep  = clock.TicksPerHour // a landed pod's dust and sound
	podTries = 40                 // landing spots tried per pod
)

// Drop plans group gi's pods for kit, arriving by a at now: one pod per
// raider. It reports where the raid is announced and how many pods come.
func (p *Pods) Drop(gi int, kit []item.Kind, a raiddef.Arrival, now uint64) (x, y, n int) {
	h := p.h
	grp := &h.Groups()[gi]
	grp.Arrival = a
	cx, cy, centred := p.spot(a)
	x, y = cx, cy
	w, ht := h.Size()
	for _, k := range kit {
		px, py, ok := 0, 0, false
		for try := 0; try < podTries && !ok; try++ {
			if centred {
				px, py = cx+h.Roll(2*raiddef.PodRadius+1)-raiddef.PodRadius, cy+h.Roll(2*raiddef.PodRadius+1)-raiddef.PodRadius
			} else {
				px, py = h.Roll(w), h.Roll(ht)
			}
			ok = p.free(grp.Pods, px, py)
		}
		if !ok {
			continue
		}
		if n == 0 && !centred {
			x, y = px, py
		}
		grp.Pods = append(grp.Pods, Pod{X: px, Y: py, Land: now + raiddef.PodFallTicks + uint64(h.Roll(raiddef.PodStagger)), Kit: k})
		n++
	}
	return x, y, n
}

// spot is where centre and edge pods gather (centred false: scattered).
func (p *Pods) spot(a raiddef.Arrival) (int, int, bool) {
	h := p.h
	hx, hy := h.Home()
	switch a {
	case raiddef.DropPodCentre:
		if _, cx, cy, ok := h.NearestColonist(hx, hy); ok {
			return cx, cy, true
		}
		return hx, hy, true
	case raiddef.DropPodEdge:
		if ex, ey, ok := p.EdgeTile(); ok {
			ex, ey = p.inward(ex, ey)
			return ex, ey, true
		}
	}
	return hx, hy, false
}

// inward moves an edge tile PodRadius tiles into the map, so the pods
// spread round it land on the map.
func (p *Pods) inward(x, y int) (int, int) {
	w, ht := p.h.Size()
	r := raiddef.PodRadius
	return min(max(x, r), w-1-r), min(max(y, r), ht-1-r)
}

// free: a pod may land on (x, y), not beside another of its group.
func (p *Pods) free(pods []Pod, x, y int) bool {
	if !p.h.InBounds(x, y) || !p.a.PodSpot(x, y) {
		return false
	}
	for _, o := range pods {
		if weapon.Distance(o.X, o.Y, x, y) <= 1 {
			return false
		}
	}
	return true
}

// Left is how many of group gi's pods are still falling.
func (p *Pods) Left(gi int) int { return podsLeft(p.h.Groups()[gi].Pods) }

func podsLeft(pods []Pod) int {
	n := 0
	for _, o := range pods {
		if !o.Landed {
			n++
		}
	}
	return n
}

// Step lands the pods whose time has come and forgets those landed over
// an hour ago (the threat phase, once a tick).
func (p *Pods) Step(now uint64) {
	groups := p.h.Groups()
	for gi := range groups {
		grp := &groups[gi]
		if len(grp.Pods) == 0 {
			continue
		}
		last := uint64(0)
		for i := range grp.Pods {
			pod := &grp.Pods[i]
			if !pod.Landed && now >= pod.Land {
				if podsLeft(grp.Pods) == len(grp.Pods) {
					p.ch.Log("Hostile drop pods are landing.")
				}
				pod.Landed = true
				if grp.Faction == 0 && grp.PodFaction != 0 {
					grp.Faction = grp.PodFaction // the raid is on the map now
				}
				p.a.Land(gi, *pod)
			}
			last = max(last, pod.Land)
		}
		if podsLeft(grp.Pods) == 0 && now >= last+podKeep {
			grp.Pods = nil
		}
	}
}

// Views appends every pod still falling or landed within the hour.
func (p *Pods) Views(buf []PodView) []PodView {
	for _, grp := range p.h.Groups() {
		for _, o := range grp.Pods {
			buf = append(buf, PodView{X: o.X, Y: o.Y, Land: o.Land, Landed: o.Landed})
		}
	}
	return buf
}

// Announce is the drop-pod raid's letter text for n raiders (who names
// the faction, "" for an unattributed raid).
func Announce(a raiddef.Arrival, n int, who string) string {
	noun := "raiders"
	if n == 1 {
		noun = "raider"
	}
	if who != "" {
		noun += " of " + who
	}
	switch a {
	case raiddef.DropPodCentre:
		return fmt.Sprintf("Drop pods! %d %s are falling right on top of the colony.", n, noun)
	case raiddef.DropPodEdge:
		return fmt.Sprintf("Drop pods! %d %s are coming down together near the edge of the map.", n, noun)
	}
	return fmt.Sprintf("Drop pods! %d %s are falling all over the map.", n, noun)
}

// NearestEdge is the map-edge tile nearest (x, y) on a w by h map.
func NearestEdge(x, y, w, h int) (int, int) {
	best, bx, by := x, 0, y
	if d := w - 1 - x; d < best {
		best, bx, by = d, w-1, y
	}
	if y < best {
		best, bx, by = y, x, 0
	}
	if d := h - 1 - y; d < best {
		bx, by = x, h-1
	}
	return bx, by
}

// Warning is the threat letter of an unattributed walk-in raid of n.
func Warning(plan Plan, n int) (title, text string) {
	word := "raiders are"
	if n == 1 {
		word = "raider is"
	}
	switch plan {
	case Siege:
		return "Siege", fmt.Sprintf("Raid! %d %s coming. They look ready to dig in for a siege.", n, word)
	case Sappers:
		return "Sappers", fmt.Sprintf("Sappers! %d %s tunnelling in through the walls.", n, word)
	}
	return "Raid", fmt.Sprintf("Raid! %d %s coming for the colony.", n, word)
}
