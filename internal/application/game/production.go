package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Bill status strings for the read model.
const (
	BillActive     = ""
	BillNoInput    = "waiting for ingredients"
	BillDrying     = "drying a batch"
	BillNoFuel     = "fire needs wood"
	BillNoPower    = "no power"
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
	// Waiting: an "until you have" bill resting at its target until the
	// stock falls to ResumeAt (BillHaveEnough status).
	Waiting  bool
	ResumeAt int
}

// Recipes lists every recipe.
func Recipes() []RecipeView {
	var out []RecipeView
	for _, d := range recipe.All() {
		if d.Legacy {
			continue // older saves' recipes, upgraded on load
		}
		in := d.Input.Kind.String()
		switch {
		case d.Input.AnyRaw:
			in = "raw food"
		case d.Input.AnyStoneBlocks:
			in = "any stone blocks"
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

// electricSpeedPct is how much faster an electric bench works than its
// base kind (PLAY-021, our number).
const electricSpeedPct = 40

// electricBench reports a bench that runs on power (PLAY-021): the
// electric stove, tailoring bench and research bench. It works only while
// powered, electricSpeedPct faster.
func electricBench(k construction.Kind) bool { return power.SpecOf(k).Class == power.ClassBench }

// electricPct is kind k's extra work speed in percent (0 but for an
// electric bench).
func electricPct(k construction.Kind) int {
	if electricBench(k) {
		return electricSpeedPct
	}
	return 0
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
		if st := g.stacks.Ref(id); st != nil && st.Kind == k {
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
		if d.Name == "" || d.Bench != construction.RecipeBench(b.Kind) || g.recipeLocked(d.ID) {
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
		sp := g.stacks.Ref(iid)
		// a rotten body still burns on a pyre
		if sp == nil || sp.Carrier != 0 || sp.Forbidden || sp.Qty < in.Qty || !in.AcceptsStack(*sp) || sp.Rotten() && sp.Kind != item.CorpseHuman {
			continue
		}
		st := *sp // a candidate (the loop only reads)
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
		bp := g.bldgs.Ref(bid)
		if bp == nil || !construction.DefOf(bp.Kind).Bench {
			continue
		}
		b := *bp // a bench: copied before anything below can change the store
		if owner, taken := g.reserve.EntityOwner(bid); taken && owner != id {
			continue
		}
		if g.postponedFor(id, job.Bill, bid, 0, 0) {
			continue // SIM-027: blocked or out of reach lately
		}
		var spot [2]int // the work spot, worked out once a bill wants doing
		spotOK, unreach := false, false
		for i, bl := range g.bills[bid] {
			d := recipe.DefOf(bl.Recipe)
			if !m.Enabled(d.Skill) || !g.meetsMinSkill(id, d.Learn, d.MinSkill) { // skills.go
				continue
			}
			stock := g.billStock(d)
			bl = bl.Update(stock)
			g.bills[bid][i] = bl
			if !bl.Wants(stock + g.billPending(bl, d)) {
				continue
			}
			if d.Dry && g.racks[bid].Qty > 0 {
				continue // still drying the last batch
			}
			if b.Kind == construction.Campfire && g.fuel[bid] <= 0 {
				continue // no fire to cook over
			}
			if electricBench(b.Kind) && !g.powered(bid) {
				continue // PLAY-021: an electric bench works only on power
			}
			if d.ID == recipe.Cremate && g.freeWood() < d.ExtraQty {
				continue // no wood to burn the body with
			}
			sid, st := g.ingredientFor(id, d.Input)
			if sid == 0 {
				continue
			}
			if !spotOK {
				_, spot, _ = construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
				spotOK = true
			}
			if unreach || !g.reachable(id, spot[0], spot[1]) {
				if !unreach { // SIM-027: searched once, put off
					g.postpone(postKey{Pawn: id, Kind: job.Bill, Target: bid}, job.Unreachable)
				}
				unreach = true
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

// billPending is the output already being made for an "until you have"
// bill, so two cooks don't both start the last meal.
func (g *Game) billPending(bl recipe.Bill, d recipe.Def) int {
	if bl.Mode != recipe.UntilHave || d.Dry {
		return 0
	}
	n := 0
	for _, pid := range g.pawnIDs {
		j, ok := g.jobs.Get(pid)
		if !ok || j.Kind != job.Bill || j.State == job.Complete || j.State == job.Fail {
			continue
		}
		if bills := g.bills[j.Dest]; j.BillIndex < len(bills) && bills[j.BillIndex].Recipe == bl.Recipe {
			n += d.OutQty
		}
	}
	return n
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
		if take := d.Input.Take(st.Qty); st.Qty > take {
			st.Qty -= take
			g.stacks.Set(j.Target, st)
			nid := g.entities.Next()
			g.itemIDs = append(g.itemIDs, nid)
			g.stacks.Set(nid, item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Carrier: id, Rot: st.Rot, Dried: st.Dried,
				Of: st.Of, DiedAt: st.DiedAt, Animal: st.Animal})
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
	if electricBench(b.Kind) && !g.powered(j.Dest) {
		j.State, j.Fail = job.Fail, job.Blocked // PLAY-021: the power went (put off a while: postpone.go)
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgressPct(id, d.Learn, true, 100+g.cabinetBonus(j.Dest)+electricPct(b.Kind)) // skills.go; tool cabinets
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	if d.Dry {
		if st, ok := g.stacks.Get(j.Carry); ok {
			g.racks[j.Dest] = dryBatch{Kind: st.Kind, Qty: st.Qty, Rot: st.Rot, Left: st.Kind.DryTicks()}
		}
		g.deleteItem(j.Carry)
	} else if d.ID == recipe.Cremate {
		g.cremate(j.Dest, j.Carry, d) // burns the body and its wood (burial.go)
	} else {
		out, extra := d.OutQty, d.ExtraQty
		if d.BySpecies { // butchering: the yields come from the carcass's species
			if st, ok := g.stacks.Get(j.Carry); ok {
				out, extra = animal.Yield(animal.Species(st.Animal))
				out, extra = g.butcherYield(id, out), g.butcherYield(id, extra) // skills.go
			}
		}
		g.deleteItem(j.Carry)
		if out > 0 {
			g.dropStack(d.Output, out, j.DestX, j.DestY)
		}
		if extra > 0 {
			g.dropStack(d.Extra, extra, j.DestX, j.DestY)
		}
	}
	j.Carry = 0
	bl.Done++
	bills[j.BillIndex] = bl
	g.bills[j.Dest] = bills
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) billViews(bid entity.EntityID) []BillView {
	var out []BillView
	unpowered := false // PLAY-021: an electric bench without power
	if b := g.bldgs.Ref(bid); b != nil && electricBench(b.Kind) {
		unpowered = !g.powered(bid)
	}
	for _, bl := range g.bills[bid] {
		d := recipe.DefOf(bl.Recipe)
		st := BillActive
		stock := g.billStock(d)
		bl = bl.Update(stock) // local copy: views never mutate
		switch {
		case bl.Suspended:
			st = BillSuspended
		case !bl.Wants(stock) && bl.Mode == recipe.DoCount:
			st = BillDone
		case !bl.Wants(stock):
			st = BillHaveEnough
		case d.Dry && g.racks[bid].Qty > 0:
			st = BillDrying
		case d.Bench == construction.Campfire && g.fuel[bid] <= 0,
			d.ID == recipe.Cremate && g.freeWood() < d.ExtraQty:
			st = BillNoFuel
		case unpowered:
			st = BillNoPower
		default:
			if sid, _ := g.ingredientFor(0, d.Input); sid == 0 {
				st = BillNoInput
			}
		}
		out = append(out, BillView{
			Recipe: d.Name, Label: d.Label, Mode: bl.Mode.String(), ModeID: uint8(bl.Mode),
			Target: bl.Target, Done: bl.Done, Suspended: bl.Suspended, Status: st,
			Waiting: bl.Waiting, ResumeAt: bl.ResumeAt(),
		})
	}
	return out
}

// defaultBills gives new benches the bills nearly everyone wants: a
// butcher table butchers every carcass type and a drying rack dries any
// perishable food, forever; a campfire cooks until there are 10 meals; a
// funeral pyre cremates every body brought to it.
func (g *Game) defaultBills(id entity.EntityID, k construction.Kind) {
	k = construction.RecipeBench(k) // PLAY-021: an electric bench gets its base kind's bills
	if k == construction.Pyre {
		g.bills[id] = append(g.bills[id], recipe.Bill{Recipe: recipe.Cremate, Mode: recipe.Forever, Target: 1})
		return
	}
	if k == construction.Campfire {
		g.bills[id] = append(g.bills[id], recipe.Bill{Recipe: recipe.CampfireMeal, Mode: recipe.UntilHave, Target: 10})
		return
	}
	if k != construction.ButcherTable && k != construction.DryingRack {
		return
	}
	for _, d := range recipe.For(k) {
		g.bills[id] = append(g.bills[id], recipe.Bill{Recipe: d.ID, Mode: recipe.Forever, Target: 1})
	}
}
