package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Bill status strings for the read model.
const (
	BillActive     = ""
	BillNoInput    = "waiting for ingredients"
	BillDone       = "done"
	BillSuspended  = "suspended"
	BillHaveEnough = "have enough"
)

// RecipeView describes a recipe for the bills UI.
type RecipeView struct {
	ID     uint8
	Name   string
	Label  string
	Bench  string
	Input  string // e.g. "2 raw food"
	Output string
}

// BillView is a bench bill in the read model.
type BillView struct {
	Recipe    string
	Label     string
	Mode      string
	ModeID    uint8
	Target    int
	Done      int
	Suspended bool
	Status    string
}

// Recipes lists every recipe.
func Recipes() []RecipeView {
	var out []RecipeView
	for _, d := range recipe.All() {
		in := d.Input.Kind.String()
		if d.Input.AnyRaw {
			in = "raw food"
		}
		out = append(out, RecipeView{
			ID: uint8(d.ID), Name: d.Name, Label: d.Label, Bench: d.Bench.String(),
			Input: itoa(d.Input.Qty) + " " + in, Output: d.Output.String(),
		})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// benchAt is the completed bench building covering (x, y).
func (g *Game) benchAt(x, y int) (entity.EntityID, construction.Building, bool) {
	bid := g.layerBldgAt(x, y, construction.LayerObject)
	b, ok := g.bldgs.Get(bid)
	if !ok || !construction.DefOf(b.Kind).Bench {
		return 0, construction.Building{}, false
	}
	return bid, b, true
}

func (g *Game) stockOf(k item.Kind) int {
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

func (g *Game) applyBill(c command.Command) {
	bid, b, ok := g.benchAt(c.X, c.Y)
	if !ok {
		return
	}
	bills := g.bills[bid]
	switch c.Kind {
	case command.AddBill:
		d := recipe.DefOf(recipe.ID(c.Recipe))
		if d.Name == "" || d.Bench != b.Kind {
			return
		}
		target := c.Count
		if target <= 0 {
			target = 1
		}
		bills = append(bills, recipe.Bill{Recipe: d.ID, Mode: recipe.Mode(c.Mode), Target: target})
	case command.SetBill:
		if c.Index < 0 || c.Index >= len(bills) {
			return
		}
		bl := bills[c.Index]
		bl.Mode = recipe.Mode(c.Mode)
		if c.Count > 0 {
			bl.Target = c.Count
		}
		bl.Suspended = c.Enabled
		bills[c.Index] = bl
	case command.RemoveBill:
		if c.Index < 0 || c.Index >= len(bills) {
			return
		}
		bills = append(bills[:c.Index:c.Index], bills[c.Index+1:]...)
	}
	g.bills[bid] = bills
}

// ingredientFor is the nearest free stack that can feed in on its own.
func (g *Game) ingredientFor(id entity.EntityID, in recipe.Input) (entity.EntityID, item.Stack) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	var bst item.Stack
	bestD := 1 << 30
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || st.Carrier != 0 || st.Qty < in.Qty || !in.Accepts(st.Kind) {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(iid); taken && owner != id {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if d < bestD || d == bestD && iid < best {
			best, bst, bestD = iid, st, d
		}
	}
	return best, bst
}

func (g *Game) assignBill(id entity.EntityID, m work.Mask) bool {
	for _, bid := range g.bldgIDs {
		b, ok := g.bldgs.Get(bid)
		if !ok || !construction.DefOf(b.Kind).Bench {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for i, bl := range g.bills[bid] {
			d := recipe.DefOf(bl.Recipe)
			if !m.Enabled(d.Skill) || !bl.Wants(g.stockOf(d.Output)) {
				continue
			}
			sid, st := g.ingredientFor(id, d.Input)
			if sid == 0 {
				continue
			}
			if !g.reachable(id, spot[0], spot[1]) {
				continue
			}
			rec := job.Record{Kind: job.Bill, Target: sid, TX: st.X, TY: st.Y, Dest: bid,
				DestX: spot[0], DestY: spot[1], WorkLeft: d.Work, BillIndex: i}
			if !g.startJob(id, rec) {
				continue
			}
			g.reserve.TryEntity(id, bid)
			return true
		}
	}
	return false
}

func (g *Game) doBill(id entity.EntityID, j job.Record) {
	b, ok := g.bldgs.Get(j.Dest)
	bills := g.bills[j.Dest]
	if !ok || j.BillIndex >= len(bills) {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	bl := bills[j.BillIndex]
	d := recipe.DefOf(bl.Recipe)
	if j.Carry == 0 {
		st, ok := g.stacks.Get(j.Target)
		if !ok || st.Carrier != 0 || st.Qty < d.Input.Qty {
			j.State, j.Fail = job.Fail, job.TargetGone
			g.jobs.Set(id, j)
			return
		}
		carry := j.Target
		if st.Qty > d.Input.Qty {
			st.Qty -= d.Input.Qty
			g.stacks.Set(j.Target, st)
			nid := g.entities.Next()
			g.itemIDs = append(g.itemIDs, nid)
			g.stacks.Set(nid, item.Stack{Kind: st.Kind, Qty: d.Input.Qty, X: st.X, Y: st.Y, Carrier: id})
			carry = nid
		} else {
			st.Carrier = id
			g.stacks.Set(j.Target, st)
		}
		j.Carry = carry
		path, ok := g.pathTo(id, j.DestX, j.DestY)
		if !ok {
			j.State, j.Fail = job.Fail, job.Unreachable
			g.jobs.Set(id, j)
			return
		}
		j.TX, j.TY = j.DestX, j.DestY
		j.Path, j.PathI = path, 0
		if len(path) > 0 {
			j.State = job.Goto
		}
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.deleteItem(j.Carry)
	j.Carry = 0
	g.dropStack(d.Output, d.OutQty, j.DestX, j.DestY)
	bl.Done++
	bills[j.BillIndex] = bl
	g.bills[j.Dest] = bills
	_ = b
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) billViews(bid entity.EntityID) []BillView {
	var out []BillView
	for _, bl := range g.bills[bid] {
		d := recipe.DefOf(bl.Recipe)
		st := BillActive
		stock := g.stockOf(d.Output)
		switch {
		case bl.Suspended:
			st = BillSuspended
		case !bl.Wants(stock) && bl.Mode == recipe.DoCount:
			st = BillDone
		case !bl.Wants(stock):
			st = BillHaveEnough
		default:
			if sid, _ := g.ingredientFor(0, d.Input); sid == 0 {
				st = BillNoInput
			}
		}
		out = append(out, BillView{
			Recipe: d.Name, Label: d.Label, Mode: bl.Mode.String(), ModeID: uint8(bl.Mode),
			Target: bl.Target, Done: bl.Done, Suspended: bl.Suspended, Status: st,
		})
	}
	return out
}
