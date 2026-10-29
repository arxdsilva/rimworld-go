package craft

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
)

// facility is one kind of building that links to nearby benches and
// speeds their work (PRD-3 generalises NG-1's tool cabinet): Pct percent
// per linked piece, at most Max of them, within Reach Chebyshev tiles.
type facility struct {
	Kind            construction.Kind
	Pct, Max, Reach int
}

// facilities is every bench facility. wiki: Tool cabinet (+6 % work
// speed each, at most two); the reach is ours (the wiki gives none).
var facilities = [...]facility{
	{Kind: construction.ToolCabinet, Pct: 6, Max: 2, Reach: 6},
}

// links reports a bench of kind k takes facilities: every bench but the
// spots (wiki: a crafting or butcher spot links nothing).
func links(k construction.Kind) bool {
	return k != construction.CraftingSpot && k != construction.ButcherSpot
}

// Facilities is what each bench links (BuildingView.Linked) and the work
// bonus that gives, cached per bench for one StructVer; never saved.
type Facilities struct {
	h   Host
	ver uint64
	by  map[entity.EntityID]linked
}

// linked is one bench's links: the bonus in percent and the linked
// pieces' names in building order (shared by every view until the
// buildings change).
type linked struct {
	pct   int
	names []string
}

// Bonus is the percent bill-work bonus of bench from linked facilities.
func (f *Facilities) Bonus(bench entity.EntityID) int { return f.of(bench).pct }

// Linked is the names of the facilities bench links, in building order
// (nil: none). The slice is shared: callers never write it.
func (f *Facilities) Linked(bench entity.EntityID) []string { return f.of(bench).names }

func (f *Facilities) of(bench entity.EntityID) linked {
	if ver := f.h.StructVer(); f.by == nil || f.ver != ver {
		f.by, f.ver = map[entity.EntityID]linked{}, ver
	}
	if l, ok := f.by[bench]; ok {
		return l
	}
	l := f.count(bench)
	f.by[bench] = l
	return l
}

// count walks the buildings once for every facility near bench.
func (f *Facilities) count(bench entity.EntityID) linked {
	var l linked
	b, ok := f.h.Bench(bench)
	if !ok || !links(b.Kind) {
		return l
	}
	var n [len(facilities)]int
	for _, bid := range f.h.BuildingIDs() {
		o, ok := f.h.Building(bid)
		if !ok {
			continue
		}
		for i, fc := range facilities {
			if o.Kind != fc.Kind || n[i] >= fc.Max || max(abs(o.X-b.X), abs(o.Y-b.Y)) > fc.Reach {
				continue
			}
			n[i]++
			l.pct += fc.Pct
			l.names = append(l.names, construction.DefOf(fc.Kind).Name)
		}
	}
	return l
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
