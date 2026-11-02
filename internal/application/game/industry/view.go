package industry

import (
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// View fills A4's fields of building id's view (axview.Building: Cycle,
// Ore, Radius, Flow); a building of another kind is left alone.
func (s *State) View(h Host, v *axview.Building, id entity.EntityID, b construction.Building) {
	switch b.Kind {
	case construction.DeepDrill:
		v.Cycle, v.Ore = s.Ops(h).Cycle(id, b.Kind), s.Ops(h).DrillOre(b)
	case construction.GroundScanner:
		v.Cycle = s.Ops(h).Cycle(id, b.Kind)
	case construction.MoisturePump:
		v.Radius = s.Pumps(h).Radius(id)
	case construction.Watermill:
		v.Flow = s.Mill(h).Flow(b)
	}
}

// DeepOre is the found deep ore for the world view (WorldView.AX:
// DeepOreVer, DeepOre) on a w-wide map, in cell order. The slice is
// shared and read only; a change hands out a fresh one.
func (s *State) DeepOre(w int) (uint64, []axview.Deposit) {
	if s.depOK && s.depVer == s.ver && s.depW == w {
		return s.ver, s.dep
	}
	s.dep, s.depVer, s.depW, s.depOK = nil, s.ver, w, true
	for _, c := range s.ore {
		if w > 0 {
			s.dep = append(s.dep, axview.Deposit{X: int(c.I) % w, Y: int(c.I) / w, Ore: item.Kind(c.Ore).String(), Left: int(c.Left)})
		}
	}
	return s.ver, s.dep
}
