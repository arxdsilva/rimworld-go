package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Research. The player picks a project; colonists with Research work sit
// at a research bench and put a point a working tick into it. Finished
// projects unlock buildings, crops and recipes. Sandbox games (New) start
// with everything researched; colonies (NewColony) start with nothing.

// researchSession is how long one sitting at the bench lasts.
const researchSession = 2 * clock.TicksPerHour

// ResearchView is one project for the Research tab.
type ResearchView struct {
	ID        uint8
	Name      string
	Label     string
	Desc      string
	Cost      int
	Progress  int
	Done      bool
	Available bool
	Current   bool
	Requires  []string // labels of unfinished prerequisites
	Unlocks   []string // player-facing: "drying rack", "healroot crop", ...
}

func (g *Game) researchViews() []ResearchView {
	var out []ResearchView
	for _, d := range research.All() {
		v := ResearchView{ID: uint8(d.Project), Name: d.Name, Label: d.Label, Desc: d.Desc, Cost: d.Cost,
			Progress: g.research[d.Project], Done: g.research.Done(d.Project),
			Available: g.research.Available(d.Project), Current: g.researchSet && g.researchCur == d.Project}
		for _, r := range d.Requires {
			if !g.research.Done(r) {
				v.Requires = append(v.Requires, research.DefOf(r).Label)
			}
		}
		for _, u := range d.Unlocks {
			v.Unlocks = append(v.Unlocks, unlockLabel(u))
		}
		out = append(out, v)
	}
	return out
}

// unlockLabel turns an unlock key into words.
func unlockLabel(key string) string {
	for _, p := range []struct{ prefix, suffix string }{{"build:", ""}, {"crop:", " crop"}, {"recipe:", ""}} {
		if len(key) > len(p.prefix) && key[:len(p.prefix)] == p.prefix {
			name := key[len(p.prefix):]
			if p.prefix == "recipe:" {
				for _, r := range recipe.All() {
					if r.Name == name {
						return r.Label
					}
				}
			}
			return name + p.suffix
		}
	}
	return key
}

// lockedBy names the project gating an unlock key ("" when usable).
func (g *Game) lockedBy(key string) string {
	if p, ok := g.research.Locks(key); ok {
		return research.DefOf(p).Label
	}
	return ""
}

// Locked maps every still-locked unlock key to the project that opens it
// (the UI greys these out).
func (g *Game) lockedKeys() map[string]string {
	out := map[string]string{}
	for _, d := range research.All() {
		if g.research.Done(d.Project) {
			continue
		}
		for _, u := range d.Unlocks {
			out[u] = d.Label
		}
	}
	return out
}

func (g *Game) buildLocked(k construction.Kind) bool { return g.lockedBy("build:"+k.String()) != "" }

func (g *Game) cropLocked(s plant.Species) bool { return g.lockedBy("crop:"+plant.DefOf(s).Name) != "" }

func (g *Game) recipeLocked(id recipe.ID) bool {
	return g.lockedBy("recipe:"+recipe.DefOf(id).Name) != ""
}

// setResearch picks the current project (ignored unless available).
func (g *Game) setResearch(p research.Project) {
	if !g.research.Available(p) {
		return
	}
	g.researchCur, g.researchSet = p, true
}

// assignResearch sends a researcher to the nearest free research bench.
func (g *Game) assignResearch(id entity.EntityID) bool {
	if !g.researchSet || !g.research.Available(g.researchCur) {
		return false
	}
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	var bx, by int
	bestD := 1 << 30
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || b.Kind != construction.ResearchBench || b.Deconstruct {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		_, spot, ok := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		if !ok || !g.grid.Walkable(spot[0], spot[1]) {
			continue
		}
		if d := manhattan(pos.X, pos.Y, spot[0], spot[1]); d < bestD || (d == bestD && bid < best) {
			best, bestD, bx, by = bid, d, spot[0], spot[1]
		}
	}
	if best == 0 {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Research, Target: best, TX: bx, TY: by, WorkLeft: researchSession})
}

func (g *Game) doResearch(id entity.EntityID, j job.Record) {
	if _, ok := g.bldgs.Get(j.Target); !ok || !g.researchSet {
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	if g.workTick(id) && g.research.Add(g.researchCur, 1) {
		d := research.DefOf(g.researchCur)
		pos, _ := g.positions.Get(id)
		msg := "Research finished: " + d.Label + "."
		if len(d.Unlocks) > 0 {
			msg += " Unlocked:"
			for i, u := range d.Unlocks {
				if i > 0 {
					msg += ","
				}
				msg += " " + unlockLabel(u)
			}
			msg += "."
		}
		g.message(pos.X, pos.Y, "%s", msg)
		g.researchSet = false
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft <= 0 {
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
}

// CompleteResearch finishes every project (dev hook).
func (g *Game) CompleteResearch() { g.research = research.Complete() }

// AddResearch puts points into a project by name (dev scenarios).
func (g *Game) AddResearch(name string, pts int) {
	if p, ok := research.Parse(name); ok {
		g.research.Add(p, pts)
	}
}

// PickResearch sets the current project by name (dev scenarios).
func (g *Game) PickResearch(name string) {
	if p, ok := research.Parse(name); ok {
		g.setResearch(p)
	}
}

// ResetResearch forgets all research (fixtures: a fresh colony's tree).
func (g *Game) ResetResearch() { g.research, g.researchSet = research.Progress{}, false }
