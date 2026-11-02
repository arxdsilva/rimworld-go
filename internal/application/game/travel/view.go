package travel

import (
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// View fills building id's AX fields for travel's kinds (b is it). It
// writes nothing for any other kind: the ship casket's sleeper fields are
// A8's (AX-0 contract).
func (o Ops) View(ax *axview.Building, id entity.EntityID, b construction.Building) {
	switch b.Kind {
	case construction.StasisCasket:
		ax.SleeperID = uint64(o.s.caskets[id])
		ax.Sleeper = ""
		if p := o.s.caskets[id]; p != 0 {
			ax.Sleeper = o.h.Name(p)
		}
	case construction.TransportPod, construction.PodLauncher:
		gr, ok := pods(o).groupOf(id)
		if !ok {
			ax.FuelUnits = o.h.Fuel(id)
			return
		}
		ax.Group = uint64(gr.leader)
		ax.LoadKg, ax.CapKg = pods(o).loadKg(gr), capKg(gr)
		ax.Range, ax.FuelUnits = pods(o).rangeOf(gr)
		if b.Kind == construction.PodLauncher {
			ax.FuelUnits = o.h.Fuel(id)
		}
	case construction.MineralScanner:
		ax.Cycle = scanner(o).Cycle(id)
		ax.ScanFor = ""
		if k := scanner(o).ScanFor(id); k != 0 {
			ax.ScanFor = k.String()
		}
	}
}

// Wanted is pod group pod's manifest still to load: colonists, animals
// and goods left (counts), for the pod pane.
func (o Ops) Wanted(pod entity.EntityID) (pawns, animals, goods int) {
	gr, ok := pods(o).groupOf(pod)
	if !ok || o.s.wants[gr.leader] == nil {
		return 0, 0, 0
	}
	m := o.s.wants[gr.leader]
	for _, w := range m.Items {
		goods += w.Qty
	}
	return len(m.Pawns), len(m.Animals), goods
}

// World fills the world view's pods in flight and mineral sites.
func (o Ops) World(w *axview.World) {
	w.Pods, w.Sites = nil, nil
	for _, f := range o.s.flights {
		w.Pods = append(w.Pods, axview.PodFlight{From: int32(f.From), To: int32(f.To), Arrive: f.Arrive, Count: f.Pods})
	}
	for _, s := range o.s.sites {
		w.Sites = append(w.Sites, axview.Site{Tile: int32(s.Tile), Mineral: s.Mineral.String(), Expires: s.Expires})
	}
}

// Option is one order-menu row (the game's OrderOption, field for field).
type Option struct {
	Label    string
	Cmd      command.Command
	Disabled bool
	Reason   string
}

// Orders are pawn's stasis casket rows for a click on (x, y): enter the
// casket there, or carry the downed pawn lying there to the nearest
// empty casket.
func (o Ops) Orders(pawn entity.EntityID, x, y int) []Option {
	for _, bid := range o.h.BuildingIDs() {
		if b, ok := o.h.Building(bid); ok && b.Kind == construction.StasisCasket && construction.Covers(b.Kind, b.Facing, b.X, b.Y, x, y) {
			opt := Option{Label: "Enter stasis casket", Cmd: command.Command{Kind: command.EnterCasket, Pawn: uint64(pawn), Target: uint64(bid)}}
			if p := o.s.caskets[bid]; p != 0 {
				opt.Disabled, opt.Reason = true, "occupied by "+o.h.Name(p)
			}
			return []Option{opt}
		}
	}
	for _, pid := range o.h.PawnIDs() {
		if pid == pawn || !o.h.Downed(pid) {
			continue
		}
		if px, py := o.h.Pos(pid); px != x || py != y {
			continue
		}
		casket := caskets(o).nearestCasket(x, y)
		opt := Option{Label: "Carry " + o.h.Name(pid) + " to stasis casket",
			Cmd: command.Command{Kind: command.EnterCasket, Pawn: uint64(pawn), Target: uint64(casket), Worker: uint64(pid)}}
		if casket == 0 {
			opt.Disabled, opt.Reason = true, "no empty stasis casket"
		}
		return []Option{opt}
	}
	return nil
}

// nearestCasket is the empty stasis casket nearest (x, y) (0: none).
func (o caskets) nearestCasket(x, y int) entity.EntityID {
	var best entity.EntityID
	bd := 1 << 30
	for _, id := range o.h.BuildingIDs() {
		b, ok := o.h.Building(id)
		if !ok || b.Kind != construction.StasisCasket || !o.freeCasket(id) {
			continue
		}
		if d := abs(b.X-x) + abs(b.Y-y); d < bd {
			best, bd = id, d
		}
	}
	return best
}
