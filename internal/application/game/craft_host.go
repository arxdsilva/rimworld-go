package game

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Bill status strings for the read model (craft/bills.go decides them).
const (
	BillActive     = craft.BillActive
	BillNoInput    = craft.BillNoInput
	BillDrying     = craft.BillDrying
	BillNoFuel     = craft.BillNoFuel
	BillNoPower    = craft.BillNoPower
	BillDone       = craft.BillDone
	BillSuspended  = craft.BillSuspended
	BillHaveEnough = craft.BillHaveEnough
)

// The bills read model lives in craft (craft/view.go); these names keep
// it in the game's API.
type (
	RecipeView         = craft.RecipeView
	BillView           = craft.BillView
	BillIngredientView = craft.BillIngredientView
	BillIngredients    = craft.BillIngredients
)

// Recipes lists every recipe.
func Recipes() []RecipeView { return craft.Recipes() }

// stockOf is the colony's count of kind k.
func (g *Game) stockOf(k item.Kind) int {
	n := 0
	for _, id := range g.itemIDs {
		if st := g.stacks.Ref(id); st != nil && st.Kind == k {
			n += st.Qty
		}
	}
	return n
}

// ingredientFor is the nearest free stack that can feed in on its own
// (bills, refuelling, surgery, medicine).
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

// dropStackOf spawns a copy of st (kind, quantity, rot, dried, found and
// marked) at (x, y), merging into a matching stack there.
func (g *Game) dropStackOf(st item.Stack, x, y int) {
	sid, err := g.SpawnStack(st.Kind, st.Qty, x, y)
	if err != nil {
		return
	}
	if s, ok := g.stacks.Get(sid); ok {
		s.Rot, s.Dried = st.Rot, st.Dried
		s.Unclaimed, s.HaulMarked = st.Unclaimed, st.HaulMarked
		g.mergeOrSet(sid, s)
	}
}

// craftHost implements craft.Host (craft/deps.go) over the game: one small
// adapter type per role, each a thin view of *Game, so no method is added
// to Game itself (.cursor/SOLID_GO.md §10).
type craftHost struct {
	craftBenches
	craftStock
	craftOutput
	craftWorkers
	craftCrafters
}

var _ craft.Host = craftHost{}

func newCraftHost(g *Game) craftHost {
	return craftHost{craftBenches{g}, craftStock{g}, craftOutput{g}, craftWorkers{g}, craftCrafters{g}}
}

type craftBenches struct{ g *Game }

func (h craftBenches) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h craftBenches) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h craftBenches) Bench(id entity.EntityID) (construction.Building, bool) {
	b := h.g.bldgs.Ref(id)
	if b == nil || !construction.DefOf(b.Kind).Bench {
		return construction.Building{}, false
	}
	return *b, true
}
func (h craftBenches) ObjectAt(x, y int) entity.EntityID {
	return h.g.layerBldgAt(x, y, construction.LayerObject)
}
func (h craftBenches) StructVer() uint64               { return h.g.structVer }
func (h craftBenches) Powered(id entity.EntityID) bool { return h.g.powered(id) }
func (h craftBenches) Fuel(id entity.EntityID) int     { return h.g.fuel[id] }
func (h craftBenches) TemperatureAt(x, y int) int      { return int(h.g.temperatureAt(x, y)) }

type craftStock struct{ g *Game }

func (h craftStock) Stock(k item.Kind) int { return h.g.stockOf(k) }
func (h craftStock) DriedStock() int {
	n := 0
	for _, id := range h.g.itemIDs {
		if st, ok := h.g.stacks.Get(id); ok && st.Dried {
			n += st.Qty
		}
	}
	return n
}
func (h craftStock) FreeWood() int { return h.g.freeWood() }
func (h craftStock) Ingredient(id entity.EntityID, in recipe.Input) (entity.EntityID, item.Stack) {
	return h.g.ingredientFor(id, in)
}
func (h craftStock) Stack(id entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(id) }
func (h craftStock) ItemIDs() []entity.EntityID                  { return h.g.itemIDs }
func (h craftStock) Merge(into, from entity.EntityID) {
	a, b := h.g.stacks.Get(into)
	if fs, ok := h.g.stacks.Get(from); b && ok {
		a.Rot, a.Qty = item.MergeRot(a.Qty, a.Rot, fs.Qty, fs.Rot), a.Qty+fs.Qty
		h.g.stacks.Set(into, a)
		h.g.deleteItem(from)
	}
}
func (h craftStock) Carry(carrier, stack entity.EntityID, take int) entity.EntityID {
	g := h.g
	st, _ := g.stacks.Get(stack)
	if st.Qty <= take {
		st.Carrier = carrier
		g.stacks.Set(stack, st)
		return stack
	}
	st.Qty -= take
	g.stacks.Set(stack, st)
	nid := g.entities.Next()
	g.itemIDs = append(g.itemIDs, nid)
	g.stacks.Set(nid, item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Carrier: carrier, Rot: st.Rot, Dried: st.Dried,
		Of: st.Of, DiedAt: st.DiedAt, Animal: st.Animal})
	return nid
}

type craftOutput struct{ g *Game }

func (h craftOutput) DeleteItem(id entity.EntityID)        { h.g.deleteItem(id) }
func (h craftOutput) DropStack(k item.Kind, qty, x, y int) { h.g.dropStack(k, qty, x, y) }
func (h craftOutput) DropStackOf(st item.Stack, x, y int)  { h.g.dropStackOf(st, x, y) }
func (h craftOutput) DropMeal(cook entity.EntityID, bench construction.Kind, k item.Kind, qty, x, y int) {
	h.g.dropMeal(k, qty, x, y, h.g.mealPoison(cook, bench, x, y)) // food_poison.go (WS-M)
}
func (h craftOutput) Cremate(pyre, body entity.EntityID, d recipe.Def) { h.g.cremate(pyre, body, d) } // burial.go
func (h craftOutput) Message(x, y int, text string)                    { h.g.message(x, y, "%s", text) }
func (h craftOutput) PutDown(id entity.EntityID, x, y int) {
	if st, ok := h.g.stacks.Get(id); ok {
		st.Carrier, st.X, st.Y = 0, x, y
		h.g.stacks.Set(id, st)
	}
}
func (h craftOutput) Store(carrier entity.EntityID, k item.Kind, x, y int, z uint64) bool {
	g := h.g
	sid := g.itemAt(x, y)
	st, ok := g.stacks.Get(sid)
	if !ok || st.Kind != k {
		return false
	}
	dx, dy, ok := g.areaStockpileCell(carrier, k, x, y) // area_dest.go
	if z != 0 {
		dx, dy, ok = g.stockpileCellWhere(k, x, y, func(cx, cy int) bool { id, in := g.zones.At(cx, cy); return in && uint64(id) == z })
	}
	return ok && g.startJob(carrier, job.Record{Kind: job.Haul, Target: sid, TX: x, TY: y, DestX: dx, DestY: dy})
}

type craftWorkers struct{ g *Game }

func (h craftWorkers) PawnIDs() []entity.EntityID                       { return h.g.pawnIDs }
func (h craftWorkers) Job(id entity.EntityID) (job.Record, bool)        { return h.g.jobs.Get(id) }
func (h craftWorkers) SetJob(id entity.EntityID, j job.Record)          { h.g.jobs.Set(id, j) }
func (h craftWorkers) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h craftWorkers) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}
func (h craftWorkers) Reachable(id entity.EntityID, x, y int) bool { return h.g.reachable(id, x, y) }
func (h craftWorkers) Owner(target entity.EntityID) (entity.EntityID, bool) {
	return h.g.reserve.EntityOwner(target)
}
func (h craftWorkers) Claim(owner, target entity.EntityID) bool {
	return h.g.reserve.TryEntity(owner, target)
}

type craftCrafters struct{ g *Game }

func (h craftCrafters) SkillLevel(id entity.EntityID, sk skill.ID) uint8 {
	return h.g.skillOf(id, sk).Level
}
func (h craftCrafters) BenchWork(id entity.EntityID, sk skill.ID, pct int) int {
	return h.g.workProgressPct(id, sk, true, pct) // skills.go
}
func (h craftCrafters) ButcherYield(id entity.EntityID, qty int) int {
	return h.g.butcherYield(id, qty)
}
func (h craftCrafters) Postponed(id, bench entity.EntityID) bool {
	return h.g.postponedFor(id, job.Bill, bench, 0, 0) // SIM-027
}
func (h craftCrafters) Postpone(id, bench entity.EntityID) {
	h.g.postpone(postKey{Pawn: id, Kind: job.Bill, Target: bench}, job.Unreachable)
}
func (h craftCrafters) RecipeLocked(id recipe.ID) bool { return h.g.recipeLocked(id) }
func (h craftCrafters) Now() uint64                    { return uint64(h.g.clock.Now()) }
func (h craftCrafters) Name(id entity.EntityID) string {
	if !h.g.colonist(id) || !h.g.alive(id) {
		return ""
	}
	return h.g.nameOf(id)
}
