package game

import (
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/kit"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/vault"
)

// Sleeper caskets (MAPGEN-3 §4.2). A vault's caskets are sealed ancient
// pieces (construction.SleeperCasket, Occupant = a vault.Occupant,
// CasketGroup = the vault id). They open together: the "Open casket"
// order (command.OpenCasket) has a colonist walk over and work
// casketOpenWork ticks, and any damage to a sealed casket opens it at
// once. Opening one opens every sealed casket of its group, each
// releasing its occupant; the open shell is then salvaged like any
// placed piece (steel).
//
// The order rides the deconstruct job: a sealed casket marked Deconstruct
// is worked for casketOpenWork and opened instead of taken down
// (openedCasket, from doDeconstruct). The renderer hears the opening as a
// BuildingView whose Sealed went from true to false (sfx.casket_open).

const (
	// casketOpenWork is the work of breaking a casket's seal.
	casketOpenWork = 250
	// casketSalt seeds the occupants' generated bios.
	casketSalt = 0xCA5CE7

	// ReasonCasketUnreachable refuses an Open order no colonist can carry
	// out.
	ReasonCasketUnreachable = "No colonist can reach it"

	casketJoinTitle = "A sleeper wakes and asks to stay"
	casketEmptyText = "The casket is empty."
)

// orderOpenCasket marks sealed casket id to be opened by a colonist; an
// unseen casket is ignored, an unreachable one refused with a message.
func (g *Game) orderOpenCasket(id entity.EntityID) {
	b, ok := g.bldgs.Get(id)
	if !ok || b.Kind != construction.SleeperCasket || b.Occupant == 0 || b.Deconstruct || g.fogged(b.X, b.Y) {
		return
	}
	if r := g.casketOpenReason(id); r != "" {
		g.message(b.X, b.Y, "Can't open the sleeper casket: %s.", r)
		return
	}
	b.Deconstruct, b.WorkLeft = true, casketOpenWork
	g.bldgs.Set(id, b)
}

// casketOpenReason is why the Open order on casket id would be refused
// ("": a standing colonist can reach a cell beside it).
func (g *Game) casketOpenReason(id entity.EntityID) string {
	b, ok := g.bldgs.Get(id)
	if !ok {
		return ReasonCasketUnreachable
	}
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	for _, pid := range g.pawnIDs {
		if !g.standingPawn(pid) {
			continue
		}
		for _, c := range cells {
			if _, _, ok := g.reachableSpot(pid, c[0], c[1]); ok {
				return ""
			}
		}
	}
	return ReasonCasketUnreachable
}

// openedCasket is doDeconstruct's hook: a sealed casket whose work is done
// is opened (with its group), not taken down.
func (g *Game) openedCasket(bid entity.EntityID, b construction.Building) bool {
	if b.Kind != construction.SleeperCasket || b.Occupant == 0 {
		return false
	}
	g.openCasketGroup(bid)
	return true
}

// sealedCaskets counts the sealed caskets of group.
func (g *Game) sealedCaskets(group int) int {
	n := 0
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.SleeperCasket && b.Occupant != 0 && b.CasketGroup == group {
			n++
		}
	}
	return n
}

// openCasketGroup opens sealed casket bid and every other sealed casket
// of its group, in building order, releasing each occupant. Opening a
// vault that was never breached breaches it first (its defenders wake).
func (g *Game) openCasketGroup(bid entity.EntityID) {
	first, ok := g.bldgs.Get(bid)
	if !ok || first.Kind != construction.SleeperCasket || first.Occupant == 0 {
		return
	}
	group := first.CasketGroup
	v := g.vaultOf(group)
	if v != nil && v.State < vault.Breached {
		g.breachVault(v)
	}
	var ids []entity.EntityID
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind == construction.SleeperCasket && b.Occupant != 0 && b.CasketGroup == group {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		b, _ := g.bldgs.Get(id)
		occ := vault.Occupant(b.Occupant)
		b.Occupant, b.Deconstruct, b.WorkLeft = 0, false, 0
		g.bldgs.Set(id, b)
		if o, taken := g.reserve.EntityOwner(id); taken {
			g.reserve.ReleaseEntity(o, id) // a colonist on its way finds the job done
		}
		g.releaseOccupant(occ, b, v)
	}
	if v != nil && v.State < vault.CasketsOpened {
		v.State = vault.CasketsOpened
	}
	g.structVer++ // the sprites change (closed to open)
	g.viewGen++   // view cache (indexes.go): may come between ticks
}

// casketExit is where an occupant of casket b climbs out: a free walkable
// cell beside it, inside its vault when it has one.
func (g *Game) casketExit(b construction.Building, v *vaultSave) (int, int, bool) {
	cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	var fallback [2]int
	found := false
	for _, c := range cells {
		for _, d := range [8][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			x, y := c[0]+d[0], c[1]+d[1]
			if !g.dryWalkable(x, y) || v != nil && !v.inside(x, y) {
				continue
			}
			if g.creatureOn(x, y) {
				if !found {
					fallback, found = [2]int{x, y}, true
				}
				continue
			}
			return x, y, true
		}
	}
	if found {
		return fallback[0], fallback[1], true
	}
	return g.combat.Tiles.WalkableNear(b.X, b.Y, 3)
}

// creatureOn reports a pawn, raider or animal standing on (x, y).
func (g *Game) creatureOn(x, y int) bool {
	for _, ids := range [][]entity.EntityID{g.pawnIDs, g.raiderIDs, g.animalIDs} {
		for _, id := range ids {
			if p, ok := g.positions.Get(id); ok && p.X == x && p.Y == y {
				return true
			}
		}
	}
	return false
}

// releaseOccupant lets occ out of opened casket b (of vault v, if known).
func (g *Game) releaseOccupant(occ vault.Occupant, b construction.Building, v *vaultSave) {
	switch occ {
	case vault.Grateful:
		g.casketSleeper(b, v)
	case vault.Hostile:
		if id := g.casketHostile(b, v, item.Rifle); id != 0 {
			g.threat("Casket breakout", b.X, b.Y, "%s bursts from a sleeper casket, armed and furious!", g.name(id))
			g.noteVaultHostile(v, id)
		}
	case vault.SleeperAndCrawler:
		g.casketSleeper(b, v)
		if id := g.casketBeast(b, v, animal.HiveCrawler); id != 0 {
			g.threat("Casket breakout", b.X, b.Y, "A hive crawler uncurls from the sleeper's casket!")
		}
	case vault.Beetles:
		n := 0
		for range 2 {
			if g.casketBeast(b, v, animal.HiveBeetle) != 0 {
				n++
			}
		}
		if n > 0 {
			g.threat("Casket breakout", b.X, b.Y, "Hive beetles scuttle out of the casket!")
		}
	case vault.Empty:
		g.message(b.X, b.Y, casketEmptyText)
	case vault.Remains:
		g.casketRemains(b, v)
	}
}

// casketBio is a casket occupant's generated identity.
func (g *Game) casketBio(tech backstory.Tag) pawngen.Pawn {
	return pawngen.Generate(g.vaultSeed()^casketSalt, int(g.entities.Peek()), 0, pawngen.Rules{Tech: tech})
}

// casketSleeper: a grateful sleeper wakes dazed and joins the colony
// (wandererJoins' path).
func (g *Game) casketSleeper(b construction.Building, v *vaultSave) {
	x, y, ok := g.casketExit(b, v)
	if !ok {
		return
	}
	id, err := g.spawnGenerated(g.casketBio(backstory.TagIndustrial), x, y) // skills.go
	if err != nil {
		return
	}
	g.ensurePerson(id)
	n, _ := g.need.Get(id)
	n.Rest = needs.Full * pawngen.ExhaustedRest / 100 // dazed from the long sleep
	g.need.Set(id, n)
	g.letter(letterGood, casketJoinTitle, g.personName(id)+" climbs out of the sleeper casket, dazed, and asks to stay. "+g.personName(id)+" has joined the colony.", id)
	g.addHistory("joined", g.personName(id)+" woke in a sleeper casket and joined the colony.", id)
}

// casketHostile: a one-person raider group of its own (no faction),
// carrying weapon.
func (g *Game) casketHostile(b construction.Building, v *vaultSave, weapon item.Kind) entity.EntityID {
	x, y, ok := g.casketExit(b, v)
	if !ok {
		return 0
	}
	now := uint64(g.clock.Now())
	g.raidGroups = append(g.raidGroups, raid.Group{Size: 1, Plan: raid.Assault})
	id := g.entities.Next()
	g.raiderIDs = append(g.raiderIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.equip[id] = weapon
	g.raiders[id] = raid.Raider{Group: len(g.raidGroups) - 1, Arrived: now}
	g.raiderPerson(id, 0) // people.go: a world person (relations, capture)
	if p := g.personOf(id); p != nil {
		g.names.Set(id, bioName(p.Bio))
	}
	return id
}

// casketBeast: a hostile hive insect that never calms down.
func (g *Game) casketBeast(b construction.Building, v *vaultSave, s animal.Species) entity.EntityID {
	x, y, ok := g.casketExit(b, v)
	if !ok {
		return 0
	}
	id, err := g.spawnAnimal(s, x, y)
	if err != nil {
		return 0
	}
	a, _ := g.animals.Get(id)
	a.Manhunter, a.Mode = raid.HiveForever, animal.Attack
	g.animals.Set(id, a)
	g.noteVaultHostile(v, id)
	return id
}

// casketRemains: long-dead remains (bones of a stranger) with their old
// weapon beside them.
func (g *Game) casketRemains(b construction.Building, v *vaultSave) {
	blade := kit.Casket(int(g.entities.Peek())) // domain combat/kit
	id := g.casketHostile(b, v, blade)
	if id == 0 {
		g.spawnUnclaimed(blade, 1, b.X, b.Y, false) // haul_claim.go: found, not the colony's
		g.message(b.X, b.Y, "Old remains crumble in the casket, beside their gear.")
		return
	}
	g.die(id, health.CauseNone) // combat.go: the body, the weapon beside it
	if cid, st, ok := g.bodyOf(id); ok {
		st.Kind = item.Bones // long dead
		g.putStack(cid, st)
	}
	g.message(b.X, b.Y, "Old remains lie in the casket, with their gear.")
}

// noteVaultHostile counts id among v's hostiles (for clearing).
func (g *Game) noteVaultHostile(v *vaultSave, id entity.EntityID) {
	if v != nil {
		v.Hostiles = append(v.Hostiles, uint64(id))
	}
}

// fillCasketView fills a casket's Sealed and OpenReason.
func (g *Game) fillCasketView(bv *BuildingView, id entity.EntityID, b construction.Building) {
	if b.Kind != construction.SleeperCasket || b.Occupant == 0 {
		return
	}
	bv.Sealed = true
	if !g.fogged(b.X, b.Y) {
		bv.OpenReason = g.casketOpenReason(id)
	}
}
