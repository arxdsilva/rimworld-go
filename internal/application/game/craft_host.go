package game

import (
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Bill status strings for the read model (craft/bills.go decides them).
const (
	BillActive     = craft.BillActive
	BillNoInput    = craft.BillNoInput
	BillDrying     = craft.BillDrying
	BillFermenting = craft.BillFermenting
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
	WornView           = craft.WornView // PRD-2: PawnView.Worn
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
		if g.reserve.FreeFor(id, iid, sp.Qty) < in.Qty {
			continue // RES-1: too little left that other pawns haven't claimed
		}
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if d < bestD || d == bestD && iid < best {
			best, bst, bestD = iid, st, d
		}
	}
	return best, bst
}

// dropStackOf spawns a copy of st (kind, quantity, rot, dried, found and
// marked; PRD-2: quality, wear, stuff, and the dead wearer of tainted
// apparel) at (x, y), merging into a matching stack there.
func (g *Game) dropStackOf(st item.Stack, x, y int) {
	sid, err := g.SpawnStack(st.Kind, st.Qty, x, y)
	if err != nil {
		return
	}
	if s, ok := g.stacks.Get(sid); ok {
		s.Rot, s.Dried = st.Rot, st.Dried
		s.Unclaimed, s.HaulMarked = st.Unclaimed, st.HaulMarked
		s.Quality, s.Wear, s.Stuff, s.Of = st.Quality, st.Wear, st.Stuff, st.Of
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
	craftMakers
	craftWeather
	craftShop
}

var _ craft.Host = craftHost{}

func newCraftHost(g *Game) craftHost {
	return craftHost{craftBenches{g}, craftStock{g}, craftOutput{g}, craftWorkers{g}, craftCrafters{g}, craftMakers{g}, craftWeather{g}, craftShop{g}}
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
func (h craftStock) FreeWood() int         { return h.g.freeWood() }
func (h craftStock) Ingredient(id entity.EntityID, in recipe.Input) (entity.EntityID, item.Stack) {
	return h.g.ingredientFor(id, in)
}
func (h craftStock) Stack(id entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(id) }
func (h craftStock) ItemIDs() []entity.EntityID                  { return h.g.itemIDs }
func (h craftStock) InSource(src uint64, x, y int) bool {
	z, ok := h.g.storageAt(x, y) // RES-1: the bill's storage zone, or a zone of its storage group
	return ok && (uint64(z.ID) == src || uint64(z.Group) == src)
}
func (h craftStock) SourceName(src uint64) string {
	if z, ok := h.g.zones.Zone(zone.ID(src)); ok {
		return zoneLabel(z)
	}
	return ""
}
func (h craftStock) Carry(carrier, stack entity.EntityID, take int) entity.EntityID {
	g := h.g
	st, _ := g.stacks.Get(stack)
	if st.Qty <= take {
		st.Carrier = carrier
		g.putStack(stack, st)
		return stack
	}
	st.Qty -= take
	g.putStack(stack, st)
	nid := g.entities.Next()
	g.itemIDs = append(g.itemIDs, nid)
	g.putStack(nid, item.Stack{Kind: st.Kind, Qty: take, X: st.X, Y: st.Y, Carrier: carrier, Rot: st.Rot, Dried: st.Dried,
		Of: st.Of, DiedAt: st.DiedAt, Animal: st.Animal, Quality: st.Quality, Wear: st.Wear, Stuff: st.Stuff})
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
func (h craftOutput) Store(carrier entity.EntityID, k item.Kind, x, y int, z uint64) bool {
	g := h.g
	sid := g.itemAt(x, y)
	st, ok := g.stacks.Get(sid)
	if !ok || st.Kind != k {
		return false
	}
	r := g.restrictedRef(carrier) // the bill's zone or its group (STO-2), else the nearest storage in the area; special filters too
	dx, dy, ok := g.storageCell(st, x, y, func(cx, cy int) bool {
		id, _ := g.zones.At(cx, cy)
		return z != 0 && g.zones.SameStore(id, zone.ID(z)) || z == 0 && (r == 0 || g.refContains(r, cx, cy))
	}, noPriority)
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
func (h craftWorkers) Claims() craft.Claims                        { return h.g.reserve } // never kept: replaced on load

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

// craftMakers: quality dice on the craft stream (never g.rng) and the
// creativity inspiration they spend (mind/inspiration.go, PRD-2).
type craftMakers struct{ g *Game }

func (h craftMakers) Roll(n int) int { return h.g.stream(streamCraft).IntN(n) }
func (h craftMakers) UseInspiration(id entity.EntityID) bool {
	return h.g.mind.Inspire.Use(id, mind.Creativity)
}

// craftWeather: what rot, deterioration and worn wear read (PRD-2).
type craftWeather struct{ g *Game }

func (h craftWeather) Exposed(x, y int) bool { return h.g.underSky(x, y) && !h.g.onShelf(x, y) } // PRD-4: shelves shelter
func (h craftWeather) Raining() bool {
	return h.g.weather.Kind.Falling() && !h.g.weather.Kind.Frozen()
}
func (h craftWeather) Wet(x, y int) bool {
	return h.g.isWaterAt(x, y) || h.g.grid.InBounds(x, y) && h.g.terr.At(x, y) == terrain.Marsh
}
func (h craftWeather) SetStack(id entity.EntityID, st item.Stack) { h.g.putStack(id, st) }
func (h craftWeather) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h craftWeather) Outfit(id entity.EntityID) craft.Outfit       { return h.g.apparel[id] }
func (h craftWeather) SetOutfit(id entity.EntityID, o craft.Outfit) { h.g.apparel[id] = o }

// craftShop: bench light and room, forge fuel and sculptures (PRD-3).
type craftShop struct{ g *Game }

func (h craftShop) Light(x, y int) int { return h.g.cellLightPermil(x, y) } // light.go
func (h craftShop) Cleanliness(x, y int) (int, bool) {
	g, r := h.g, h.g.roomAt(x, y)
	if r < 0 || r >= len(g.rooms) || g.underSky(x, y) {
		return 0, false
	}
	sum, _ := g.roomFilth(r) // rooms.go
	rs := g.rooms[r]
	return rs.tally.Inputs(rs.contents.Cells, rs.contents.Floored-rs.tally.Covered, sum, 0).CleanlinessHundredths, true
}
func (h craftShop) UseFuel(id entity.EntityID, work int) {
	h.g.setFuel(id, max(0, h.g.fuel[id]-work*TicksPerWood/craft.WorkPerWood)) // light.go
}
func (h craftShop) CanInstall(k construction.Kind, x, y int) bool {
	ok, _ := h.g.canPlaceAt(k, construction.South, x, y)
	return ok
}
func (h craftShop) Install(k construction.Kind, stuff item.Kind, q uint8, x, y int) entity.EntityID {
	return h.g.placeGenerated(construction.Building{Kind: k, Stuff: stuff, X: x, Y: y, Quality: q}) // setup_mapgen.go
}
