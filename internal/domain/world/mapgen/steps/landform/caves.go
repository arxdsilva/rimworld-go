package landform

import (
	"math"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/noise2"
	"rimworld-go/pkg/randx"
)

// Caves is the "caves" pass (MAPGEN-2 §2.2): writes Cave.
//
// Only with Site.Caves (never on sea ice). Rock bodies of the provisional
// mask big enough (450 cells at 250², growing with the map's area) get
// tunnels: open ones start at a cliff and head inward, closed ones start
// deep inside. A tunnel tapers from 3.5–5.5 cells to 1.3, bends smoothly
// along a noise curve, branches (two levels below the main tunnel) and
// stops at its body's edge, where its walls would thin out, on meeting
// another tunnel, or when its length budget runs out. Rock stays set
// under caves; "rocks" removes it.
type Caves struct{}

// Name is the step's stream and label key.
func (Caves) Name() string { return "caves" }

const (
	caveMin250     = 450.0 // smallest body with caves at 250², cells
	caveMinFloor   = 200   // …and on any map
	cellsPerOpen   = 900.0 // one open tunnel per this much rock (1..3)
	maxOpen        = 5
	closedFrom     = 3000.0 // bodies this big get a closed tunnel too
	closedDepth    = 6      // closed tunnels start this deep
	edgeMargin     = 2      // no cave cell this close to the map edge
	minWidth       = 1.3
	widthLo        = 4.5
	widthHi        = 7.0
	taper250       = 0.030 // width lost per step at 250² (slower on bigger maps)
	branchAfter    = 12    // steps before a tunnel may branch
	branchChance   = 0.09  // per step after that
	branchWidth    = 0.7   // a branch's width factor
	branchBudget   = 0.6   // a branch's share of the remaining budget
	maxLevel       = 2     // main tunnel 0, branches 1 and 2
	turnCell       = 6.0   // steps per turn-noise lattice cell
	turnGain       = 4.0   // turn noise gain before the clamp
	openSpacing    = 14.0  // between open-tunnel mouths in one body, cells
	closedSpacing  = 10.0
	startAttempts  = 30
	probeDist      = 6.0 // an open start must find rock this deep…
	probeDepth     = 3   // …this many cells inward
	budgetLo       = 100.0
	budgetSpan     = 70.0
	branchMinWidth = minWidth + 0.2
)

// maxTurn is the largest heading change per step: 25° per 6 steps.
var maxTurn = 40.0 / 6 * math.Pi / 180

// Run is the pass.
func (Caves) Run(c *mapgen.Ctx) error {
	r := c.Rand("caves")
	if !c.Cfg.Site.Caves || seaIce(c) {
		return nil
	}
	carve(c, r)
	return nil
}

// caveMinCells is the smallest rock body that gets caves on c's map.
func caveMinCells(c *mapgen.Ctx) int {
	s := c.Scale()
	return max(caveMinFloor, int(caveMin250*s*s))
}

// tracePt is one tunnel step: the centre, the width and the heading.
type tracePt struct{ x, y, w, head float64 }

// trace is a carved tunnel (for tests and the debug view).
type trace struct {
	open  bool
	level int
	pts   []tracePt
}

// tunnel is a tunnel waiting to be walked.
type tunnel struct {
	comp          int32
	x, y, head, w float64
	open          bool
	level, budget int
	parent        int
	graceDepth    int // steps before the wall-thickness rule applies
	graceJoin     int // steps the parent's cells don't count as a join
	phase         float64
}

// carver is one carving's shared state.
type carver struct {
	c      *mapgen.Ctx
	r      *randx.Rand
	label  []int32
	depth  []uint8
	owner  []int32
	taper  float64
	seed   uint64
	queue  []tunnel
	traces []trace
}

// carve writes F.Cave and returns the tunnels it walked.
func carve(c *mapgen.Ctx, r *randx.Rand) []trace {
	W, H := c.W, c.H
	label, sizes := mapgen.Components(W, H, c.F.Rock, false)
	minCells := caveMinCells(c)
	slot := make([]int, len(sizes))
	nq := 0
	for k, s := range sizes {
		slot[k] = -1
		if s >= minCells {
			slot[k] = nq
			nq++
		}
	}
	if nq == 0 {
		return nil
	}
	depth := mapgen.ChebDepth(W, H, c.F.Rock)
	cv := &carver{c: c, r: r, label: label, depth: depth,
		owner: make([]int32, W*H), taper: taper250 / max(1, c.Scale()), seed: c.NoiseSeed("caves/turn")}
	for i := range cv.owner {
		cv.owner[i] = -1
	}
	// Candidate starts per qualifying body, in row-major order.
	cliffs := make([][]int32, nq)
	deeps := make([][]int32, nq)
	for i, k := range label {
		if k < 0 || slot[k] < 0 || !cv.inside(i) {
			continue
		}
		q := slot[k]
		if cv.cliff(i) {
			cliffs[q] = append(cliffs[q], int32(i))
		}
		if depth[i] >= closedDepth {
			deeps[q] = append(deeps[q], int32(i))
		}
	}
	s2 := c.Scale() * c.Scale()
	for k, s := range sizes {
		q := slot[k]
		if q < 0 {
			continue
		}
		nOpen := min(maxOpen, max(1, int(float64(s)/(cellsPerOpen*s2))))
		nClosed := 0
		if float64(s) >= closedFrom*s2 {
			nClosed = 1
		}
		var starts [][2]float64
		for a := 0; a < startAttempts && len(starts) < nOpen && len(cliffs[q]) > 0; a++ {
			i := int(cliffs[q][r.IntN(len(cliffs[q]))])
			x, y := c.XY(i)
			px, py := float64(x)+0.5, float64(y)+0.5
			head := cv.inward(i)
			tx, ty := int(math.Floor(px+math.Cos(head)*probeDist)), int(math.Floor(py+math.Sin(head)*probeDist))
			if !c.In(tx, ty) || label[c.Idx(tx, ty)] != int32(k) || depth[c.Idx(tx, ty)] < probeDepth || near(starts, px, py, openSpacing) {
				continue
			}
			starts = append(starts, [2]float64{px, py})
			cv.push(tunnel{comp: int32(k), x: px, y: py, head: head, open: true, parent: -1})
		}
		closed := 0
		for a := 0; a < startAttempts && closed < nClosed && len(deeps[q]) > 0; a++ {
			i := int(deeps[q][r.IntN(len(deeps[q]))])
			x, y := c.XY(i)
			px, py := float64(x)+0.5, float64(y)+0.5
			head := r.Float64() * 2 * math.Pi
			if near(starts, px, py, closedSpacing) {
				continue
			}
			starts = append(starts, [2]float64{px, py})
			closed++
			cv.push(tunnel{comp: int32(k), x: px, y: py, head: head, parent: -1})
		}
	}
	for len(cv.queue) > 0 {
		t := cv.queue[0]
		cv.queue = cv.queue[1:]
		cv.walk(t)
	}
	return cv.traces
}

// push queues a main tunnel with its own width, budget and noise phase.
func (cv *carver) push(t tunnel) {
	t.w = widthLo + (widthHi-widthLo)*cv.r.Float64()
	t.budget = int((budgetLo + budgetSpan*cv.r.Float64()) * max(1, cv.c.Scale()))
	t.phase = cv.r.Float64() * 1000
	if t.open { // the mouth crosses thin rock before the wall rule applies
		t.graceDepth = int(t.w) + 3
	}
	cv.queue = append(cv.queue, t)
}

// near reports a start within d of (x, y).
func near(starts [][2]float64, x, y, d float64) bool {
	for _, s := range starts {
		if math.Hypot(s[0]-x, s[1]-y) < d {
			return true
		}
	}
	return false
}

// inside reports cell i far enough from the map edge for a cave.
func (cv *carver) inside(i int) bool {
	x, y := cv.c.XY(i)
	return x >= edgeMargin && y >= edgeMargin && x < cv.c.W-edgeMargin && y < cv.c.H-edgeMargin
}

// cliff reports a rock cell with open ground beside it (4-neighbours).
func (cv *carver) cliff(i int) bool {
	x, y := cv.c.XY(i)
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if cv.c.In(nx, ny) && !cv.c.F.Rock[cv.c.Idx(nx, ny)] {
			return true
		}
	}
	return false
}

// inward is the heading up the rock-depth slope at cell i.
func (cv *carver) inward(i int) float64 {
	x, y := cv.c.XY(i)
	var gx, gy float64
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if (dx == 0 && dy == 0) || !cv.c.In(x+dx, y+dy) {
				continue
			}
			g := float64(cv.depth[cv.c.Idx(x+dx, y+dy)]) - float64(cv.depth[i])
			l := math.Hypot(float64(dx), float64(dy))
			gx += g * float64(dx) / l
			gy += g * float64(dy) / l
		}
	}
	return math.Atan2(gy, gx)
}

// need is the rock depth a tunnel centre needs so a width-w tunnel keeps
// at least one cell of wall.
func need(w float64) int { return int(math.Ceil(w/2)) + 2 }

// walk carves one tunnel and queues its branches.
func (cv *carver) walk(t tunnel) {
	c := cv.c
	tid := int32(len(cv.traces))
	cv.traces = append(cv.traces, trace{open: t.open, level: t.level})
	var pts []tracePt
	x, y, head, w := t.x, t.y, t.head, t.w
	pcx, pcy := -1, -1
	for step := 0; step < t.budget; step++ {
		cx, cy := int(math.Floor(x)), int(math.Floor(y))
		if !c.In(cx, cy) {
			break
		}
		ci := c.Idx(cx, cy)
		if cv.label[ci] != t.comp || !cv.inside(ci) {
			break
		}
		if step >= t.graceDepth && int(cv.depth[ci]) < need(w) {
			break
		}
		o := cv.owner[ci]
		join := o >= 0 && o != tid && (int(o) != t.parent || step >= t.graceJoin)
		pts = append(pts, tracePt{x, y, w, head})
		cv.disc(t.comp, tid, x, y, w/2)
		if pcx >= 0 && pcx != cx && pcy != cy { // keep diagonal steps 4-connected
			if !cv.mark(t.comp, tid, cx, pcy) {
				cv.mark(t.comp, tid, pcx, cy)
			}
		}
		pcx, pcy = cx, cy
		if join {
			break
		}
		if t.level < maxLevel && step >= branchAfter && cv.r.Float64() < branchChance {
			ang := (40 + 35*cv.r.Float64()) * math.Pi / 180
			if cv.r.IntN(2) == 0 {
				ang = -ang
			}
			phase := cv.r.Float64() * 1000
			if bw := w * branchWidth; bw >= branchMinWidth {
				cv.queue = append(cv.queue, tunnel{comp: t.comp, x: x, y: y, head: head + ang, w: bw,
					level: t.level + 1, budget: int(float64(t.budget-step) * branchBudget), parent: int(tid),
					graceJoin: int(w) + 3, phase: phase})
			}
		}
		turn := (2*noise2.Gradient(cv.seed, float64(step), t.phase, turnCell) - 1) * turnGain * maxTurn
		head += max(-maxTurn, min(maxTurn, turn))
		w -= cv.taper
		if w < minWidth {
			break
		}
		x += math.Cos(head)
		y += math.Sin(head)
	}
	cv.traces[tid].pts = pts
}

// disc carves the cells whose centres lie within rad of (x, y), and the
// centre's own cell.
func (cv *carver) disc(comp, tid int32, x, y, rad float64) {
	x0, x1 := int(math.Floor(x-rad)), int(math.Floor(x+rad))
	y0, y1 := int(math.Floor(y-rad)), int(math.Floor(y+rad))
	for cy := y0; cy <= y1; cy++ {
		for cx := x0; cx <= x1; cx++ {
			dx, dy := float64(cx)+0.5-x, float64(cy)+0.5-y
			if dx*dx+dy*dy <= rad*rad {
				cv.mark(comp, tid, cx, cy)
			}
		}
	}
	cv.mark(comp, tid, int(math.Floor(x)), int(math.Floor(y)))
}

// mark carves one cell of body comp; false when it may not be carved.
func (cv *carver) mark(comp, tid int32, x, y int) bool {
	c := cv.c
	if !c.In(x, y) {
		return false
	}
	i := c.Idx(x, y)
	if cv.label[i] != comp || !cv.inside(i) {
		return false
	}
	c.F.Cave[i] = true
	if cv.owner[i] < 0 {
		cv.owner[i] = tid
	}
	return true
}

// angleDiff is a − b wrapped to (−π, π].
func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b, 2*math.Pi)
	if d > math.Pi {
		d -= 2 * math.Pi
	} else if d <= -math.Pi {
		d += 2 * math.Pi
	}
	return d
}
