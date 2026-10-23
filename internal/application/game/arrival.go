package game

import (
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
)

// Arrival (NG-1 §2.11). Drop pods: one per colonist plus one per podValue
// of kit, landing within podRadius of the map centre on dry ground,
// staggered over podStagger ticks; each leaves a pod shell, its colonist
// steps out with a rough-landing thought and its kit is dropped beside
// it. Standing: the colonists stand in a ring round the centre with the
// kit piled in the middle. Walk in: they enter at a reachable map edge and
// walk to the centre. The scenario's scattered items and starting animals
// come with them, and its intro letter is posted once everyone is down.
// All randomness is g.rng, drawn at build time.

const (
	podRadius   = 6
	podValue    = 300
	maxKitPods  = 8
	podFall     = 60  // ticks of fall streak before the first pod lands
	podStagger  = 180 // the landings spread over this many ticks
	arrivalKeep = clock.TicksPerHour
	scatterNear = 25 // "near the start" scatter radius
)

// podState is one arrival pod (saved). Pawns index Setup.Colonists.
type podState struct {
	X, Y    int
	Tick    uint64 // landing tick
	Landed  bool
	Pawns   []int
	Items   []scenario.Stack
	Animals []animal.Species
}

func clonePods(ps []podState) []podState {
	out := slices.Clone(ps)
	for i := range out {
		out[i].Pawns = slices.Clone(out[i].Pawns)
		out[i].Items = slices.Clone(out[i].Items)
		out[i].Animals = slices.Clone(out[i].Animals)
	}
	return out
}

// landColony brings the colonists, kit, scattered items and animals in.
func (g *Game) landColony() {
	cx, cy := g.grid.Width()/2, g.grid.Height()/2
	switch g.ng.fx.Arrival {
	case scenario.Standing:
		g.arriveStanding(cx, cy)
	case scenario.WalkIn:
		g.arriveWalkIn(cx, cy)
	default:
		g.planPods(cx, cy)
	}
	g.scatterKit(cx, cy)
}

// splitKit cuts the starting kit into stacks of at most podValue worth
// (at most maxKitPods pieces each), in kit order.
func splitKit(kit []scenario.Stack) []scenario.Stack {
	var out []scenario.Stack
	for _, s := range kit {
		if s.Qty <= 0 {
			continue
		}
		v := s.Kind.Value() * s.Qty
		n := min(max((v+podValue-1)/podValue, 1), maxKitPods, s.Qty)
		for i := 0; i < n; i++ {
			q := s.Qty / n
			if i == 0 {
				q += s.Qty % n
			}
			out = append(out, scenario.Stack{Kind: s.Kind, Qty: q, Near: true, Piles: 1})
		}
	}
	return out
}

// planPods places the drop pods (landing in the arrival phase).
func (g *Game) planPods(cx, cy int) {
	people := len(g.ng.setup.Colonists)
	kit := splitKit(g.ng.fx.StartItems)
	value := 0
	for _, s := range kit {
		value += s.Kind.Value() * s.Qty
	}
	kitPods := min((value+podValue-1)/podValue, maxKitPods)
	if kitPods == 0 && len(kit) > 0 && people == 0 {
		kitPods = 1
	}
	pods := make([]podState, people+kitPods)
	for i := 0; i < people; i++ {
		pods[i].Pawns = []int{i}
	}
	// the kit goes to the least loaded kit pod (the colonists' pods when
	// it is worth less than one pod)
	load := make([]int, len(pods))
	first := people
	if kitPods == 0 {
		first = 0
	}
	for _, s := range kit {
		if len(pods) == 0 {
			break
		}
		best := first
		for i := first; i < len(pods); i++ {
			if load[i] < load[best] {
				best = i
			}
		}
		pods[best].Items = append(pods[best].Items, s)
		load[best] += s.Kind.Value() * s.Qty
	}
	if len(pods) > 0 {
		pods[0].Animals = slices.Clone(g.ng.fx.Animals)
	} else {
		g.spawnTameAnimals(g.ng.fx.Animals, cx, cy)
	}
	now := uint64(g.clock.Now())
	var taken [][2]int
	for i := range pods {
		x, y, ok := g.podSpot(cx, cy, taken)
		if !ok {
			x, y = cx, cy
		}
		taken = append(taken, [2]int{x, y})
		pods[i].X, pods[i].Y = x, y
		pods[i].Tick = now + podFall + uint64(i*podStagger/max(len(pods)-1, 1))
	}
	g.ng.pods = pods
}

// podSpot is a free landing tile within podRadius of (cx, cy): dry
// ground a pod shell fits on, not next to another pod.
func (g *Game) podSpot(cx, cy int, taken [][2]int) (int, int, bool) {
	for try := 0; try < 300; try++ {
		dx, dy := g.rng.IntN(2*podRadius+1)-podRadius, g.rng.IntN(2*podRadius+1)-podRadius
		if dx*dx+dy*dy <= podRadius*podRadius && g.podFree(cx+dx, cy+dy, taken) {
			return cx + dx, cy + dy, true
		}
	}
	for r := 0; r <= 2*podRadius; r++ { // crowded: the nearest free tile
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) == r && g.podFree(cx+dx, cy+dy, taken) {
					return cx + dx, cy + dy, true
				}
			}
		}
	}
	return 0, 0, false
}

func (g *Game) podFree(x, y int, taken [][2]int) bool {
	if !g.grid.InBounds(x, y) || !g.dryWalkable(x, y) {
		return false
	}
	for _, t := range taken {
		if abs(t[0]-x) <= 1 && abs(t[1]-y) <= 1 {
			return false
		}
	}
	ok, _ := g.canPlaceAt(construction.PodShell, construction.South, x, y)
	return ok
}

// arrivalTick lands the pods whose time has come, posts the intro letter
// once everyone is down and forgets the pods an hour after the last one.
func (g *Game) arrivalTick(now uint64) {
	if g.ng.introDone && len(g.ng.pods) == 0 {
		return
	}
	all, last := true, uint64(0)
	for i := range g.ng.pods {
		p := &g.ng.pods[i]
		if !p.Landed && now >= p.Tick {
			p.Landed = true
			g.landPod(p)
		}
		all = all && p.Landed
		last = max(last, p.Tick)
	}
	if !all {
		return
	}
	if !g.ng.introDone {
		g.introLetter()
	}
	if len(g.ng.pods) > 0 && now >= last+arrivalKeep {
		g.ng.pods = nil
	}
}

// landPod: the shell, its colonist (dazed), its kit beside it.
func (g *Game) landPod(p *podState) {
	g.BuildNow(construction.DefOf(construction.PodShell).Name, item.Steel.String(), "south", p.X, p.Y)
	var used [][2]int
	for _, ci := range p.Pawns {
		if ci < 0 || ci >= len(g.ng.setup.Colonists) {
			continue
		}
		x, y, ok := g.freeLandNear(p.X, p.Y, 3, used)
		if !ok {
			continue
		}
		used = append(used, [2]int{x, y})
		if id, err := spawnGeneratedFn(g, g.ng.setup.Colonists[ci], x, y); err == nil {
			g.remember(id, mood.Landed)
		}
	}
	bx, by := p.X, p.Y
	for _, d := range [...][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		if g.dryWalkable(p.X+d[0], p.Y+d[1]) {
			bx, by = p.X+d[0], p.Y+d[1]
			break
		}
	}
	for _, s := range p.Items {
		g.dropStack(s.Kind, s.Qty, bx, by)
	}
	g.spawnTameAnimals(p.Animals, bx, by)
}

// freeLandNear is the nearest dry walkable tile to (x, y) within r that
// no colonist stands on and is not in used (ring by ring, no randomness).
func (g *Game) freeLandNear(x, y, r int, used [][2]int) (int, int, bool) {
	for d := 0; d <= r; d++ {
		for dy := -d; dy <= d; dy++ {
			for dx := -d; dx <= d; dx++ {
				if max(abs(dx), abs(dy)) != d {
					continue
				}
				nx, ny := x+dx, y+dy
				if !g.dryWalkable(nx, ny) || slices.Contains(used, [2]int{nx, ny}) || g.pawnOn(nx, ny) {
					continue
				}
				return nx, ny, true
			}
		}
	}
	return 0, 0, false
}

// pawnOn reports a colonist standing on (x, y).
func (g *Game) pawnOn(x, y int) bool {
	for _, id := range g.pawnIDs {
		if p, ok := g.positions.Get(id); ok && p.X == x && p.Y == y {
			return true
		}
	}
	return false
}

// ringOffsets are the standing arrival's places round the centre.
var ringOffsets = [...][2]int{{3, 0}, {3, 2}, {2, 3}, {0, 3}, {-2, 3}, {-3, 2}, {-3, 0}, {-3, -2}, {-2, -3}, {0, -3}, {2, -3}, {3, -2}}

// arriveStanding stands the colonists in a ring round (cx, cy), the kit
// piled in the middle.
func (g *Game) arriveStanding(cx, cy int) {
	people := g.ng.setup.Colonists
	var used [][2]int
	for i, p := range people {
		o := ringOffsets[i*len(ringOffsets)/max(len(people), 1)]
		x, y, ok := g.freeLandNear(cx+o[0], cy+o[1], 3, used)
		if !ok {
			continue
		}
		used = append(used, [2]int{x, y})
		_, _ = spawnGeneratedFn(g, p, x, y)
	}
	g.dropKit(cx, cy)
	g.spawnTameAnimals(g.ng.fx.Animals, cx, cy+1)
}

// arriveWalkIn brings the colonists in at a map edge the centre can be
// reached from; they walk to the centre, where the kit waits.
func (g *Game) arriveWalkIn(cx, cy int) {
	ex, ey, ok := g.edgeTile()
	if !ok {
		g.arriveStanding(cx, cy)
		return
	}
	var used [][2]int
	for i, p := range g.ng.setup.Colonists {
		x, y, ok := g.freeLandNear(ex, ey, 4, used)
		if !ok {
			continue
		}
		used = append(used, [2]int{x, y})
		id, err := spawnGeneratedFn(g, p, x, y)
		if err != nil {
			continue
		}
		o := ringOffsets[i%len(ringOffsets)]
		if tx, ty, ok := g.walkableNear(cx+o[0]/2, cy+o[1]/2, 3); ok {
			g.cmdMove(id, tx, ty)
		}
	}
	g.dropKit(cx, cy)
	g.spawnTameAnimals(g.ng.fx.Animals, ex, ey)
}

// dropKit piles the starting kit at (x, y) (stacks spill to free tiles).
func (g *Game) dropKit(x, y int) {
	for _, s := range g.ng.fx.StartItems {
		if s.Qty > 0 {
			g.dropStack(s.Kind, s.Qty, x, y)
		}
	}
}

// spawnTameAnimals places the colony's starting animals near (x, y).
func (g *Game) spawnTameAnimals(list []animal.Species, x, y int) {
	for _, s := range list {
		for try := 0; try < 12; try++ {
			id, err := g.spawnAnimal(s, x+g.rng.IntN(5)-2, y+g.rng.IntN(5)-2)
			if err != nil {
				continue
			}
			a, _ := g.animals.Get(id)
			a.Tame = true
			a.Name = g.freshPetName()
			g.animals.Set(id, a)
			break
		}
	}
}

// scatterKit drops the scenario's scattered items: in piles within
// scatterNear of the start, or anywhere on the map.
func (g *Game) scatterKit(cx, cy int) {
	for _, s := range g.ng.fx.Scatter {
		piles := max(s.Piles, 1)
		for i := 0; i < piles; i++ {
			q := s.Qty / piles
			if i == 0 {
				q += s.Qty % piles
			}
			if q <= 0 {
				continue
			}
			if x, y, ok := g.scatterSpot(cx, cy, s.Near); ok {
				g.dropStack(s.Kind, q, x, y)
			}
		}
	}
}

func (g *Game) scatterSpot(cx, cy int, near bool) (int, int, bool) {
	w, h := g.grid.Width(), g.grid.Height()
	for try := 0; try < 200; try++ {
		var x, y int
		if near {
			x, y = cx+g.rng.IntN(2*scatterNear+1)-scatterNear, cy+g.rng.IntN(2*scatterNear+1)-scatterNear
		} else {
			x, y = g.rng.IntN(w), g.rng.IntN(h)
		}
		if g.grid.InBounds(x, y) && g.dryWalkable(x, y) && g.itemAt(x, y) == 0 {
			return x, y, true
		}
	}
	return 0, 0, false
}

// introLetter posts the scenario's intro text once, as a letter.
func (g *Game) introLetter() {
	g.ng.introDone = true
	text := g.ng.fx.Intro
	if text == "" {
		return
	}
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: text, X: g.grid.Width() / 2, Y: g.grid.Height() / 2,
		Letter: true, Title: g.ng.setup.Scenario.Name})
}

// ArrivalView is one arrival pod for the renderer (NG-1 §2.11): it falls
// as a streak until Tick, then lands in a dust burst where its shell
// stands. Landed pods stay listed for an hour.
type ArrivalView struct {
	X, Y   int
	Tick   uint64
	Landed bool
}

func (g *Game) arrivalViews(now uint64) []ArrivalView {
	var out []ArrivalView
	for _, p := range g.ng.pods {
		if !p.Landed || now < p.Tick+arrivalKeep {
			out = append(out, ArrivalView{X: p.X, Y: p.Y, Tick: p.Tick, Landed: p.Landed})
		}
	}
	return out
}
