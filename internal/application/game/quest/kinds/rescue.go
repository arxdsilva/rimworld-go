package kinds

import (
	"strconv"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/world/mapgen"
)

// Rescue is the prisoner rescue (QST-065..069, decisions 13): a hostile
// faction holds a person in a walled cell of a small camp 6..25 tiles away.
// The quest accepts itself and runs 23 days. A caravan that reaches the
// camp opens the site: guards stand at the camp's posts and the prisoner
// stands in the cell, a guest who stays until freed. The player clears the
// guards (the cell's door is the enemy's until none stands, or a wall is
// broken: game glue), then orders a colonist to free the prisoner
// (command.OrderFreePrisoner); the game signals SigPawnFreed and the pawn
// joins. The only option is the pawn, who is paid by the freeing, not by
// the root (the quest is marked Paid then).
type Rescue struct{}

// Part indexes in a rescue quest.
const (
	rescueSite = 0
	rescuePawn = 1
)

// Layouts kept in the site part (saved: append-only): the camp's tech.
const (
	rescueTribal uint8 = iota
	rescueIndustrial
)

// Guards: one per 250 threat points, 2 at least, at most one per post
// (ours). Their weapons are the tech's, rolled one by one.
const rescueGuardPoints = 250

var (
	rescueTribalArms     = []item.Kind{item.Spear, item.ShortBow, item.Club}
	rescueIndustrialArms = []item.Kind{item.Autopistol, item.PumpShotgun, item.Revolver}
)

func (Rescue) Def() qd.Def { return qd.DefOf(qd.Rescue) }

// rescueEnemies are the hostile, undefeated factions (ascending index).
func rescueEnemies(c quest.Ctx) []int {
	var out []int
	for _, f := range c.R.World.Factions() {
		if f.Relation == faction.Hostile && !f.Defeated {
			out = append(out, f.Index)
		}
	}
	return out
}

// CanOffer: quest threats are on (the kind is violent) and a hostile
// faction can hold the prisoner.
func (Rescue) CanOffer(c quest.Ctx) bool {
	return c.R.World.QuestThreats() && len(rescueEnemies(c)) > 0
}

// Build picks the captor, a free tile 6..25 away, the camp's tech and the
// prisoner's seed.
func (Rescue) Build(c quest.Ctx, q *qd.Quest) bool {
	es := rescueEnemies(c)
	if len(es) == 0 {
		return false
	}
	enemy := es[c.Intn(len(es))]
	tile, ok := c.R.World.NearTile(c.Seed(), 6, 25)
	if !ok {
		return false
	}
	q.Giver, q.Target = 0, enemy+1
	pawn := c.Seed()
	// The pawn is the only option; the freeing pays it (see Signal).
	q.Rewards = []qd.Reward{{Pawn: pawn}}
	q.Parts = []qd.Part{
		{Kind: qd.PartSite, Tile: tile, Seed: c.Seed(), Layout: uint8(c.Intn(2))},
		{Kind: qd.PartPawn, Tile: -1, Seed: pawn},
	}
	return true
}

// Accepted claims the camp's tile on the world.
func (Rescue) Accepted(c quest.Ctx, q *qd.Quest) {
	if len(q.Parts) <= rescuePawn {
		return
	}
	p := q.Parts[rescueSite]
	c.R.Sites.Place(quest.Site{Quest: q.ID, Kind: q.Kind, Tile: p.Tile, Layout: p.Layout, Seed: p.Seed, Faction: q.Target})
}

// Signal: the site opening lays the camp, its guards and the held prisoner
// (once); a freeing succeeds; the prisoner's death, the camp's loss and the
// map closing on an unfinished rescue fail it.
func (Rescue) Signal(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	if len(q.Parts) <= rescuePawn || q.Parts[rescuePawn].Done {
		return quest.VerdictNone
	}
	site, pawn := &q.Parts[rescueSite], &q.Parts[rescuePawn]
	switch s.Kind {
	case quest.SigSiteOpened:
		if s.Tile != site.Tile || site.Map != 0 {
			return quest.VerdictNone
		}
		return rescueLay(c, q, s)
	case quest.SigPawnFreed:
		if pawn.Pawn == 0 || uint64(s.Pawn) != pawn.Pawn {
			return quest.VerdictNone
		}
		if !c.R.Pawns.Alive(s.Pawn) {
			return quest.VerdictFail(qd.PawnLost)
		}
		c.R.Pawns.Join(s.Pawn) // false when the game already joined it
		pawn.Done, site.Done, q.Paid = true, true, true
		return quest.VerdictSucceed
	case quest.SigPawnDied:
		if pawn.Pawn != 0 && uint64(s.Pawn) == pawn.Pawn {
			return quest.VerdictFail(qd.PawnLost)
		}
	case quest.SigMapClosed:
		if site.Map != 0 && s.Map == site.Map {
			return quest.VerdictFail(qd.SiteLost)
		}
	case quest.SigSiteLost:
		if s.Tile == site.Tile {
			return quest.VerdictFail(qd.SiteLost)
		}
	}
	return quest.VerdictNone
}

// rescueLay builds the camp on the open site map: the compound with its
// cell, one guard per weapon at the posts, the prisoner in the cell. When
// nothing can be laid the rescue cannot be played and fails.
func rescueLay(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	site, pawn := &q.Parts[rescueSite], &q.Parts[rescuePawn]
	b := c.R.Builder
	w, h := b.Size()
	tech, arms := faction.Neolithic, rescueTribalArms
	if site.Layout == rescueIndustrial {
		tech, arms = faction.Industrial, rescueIndustrialArms
	}
	camp, cell := mapgen.CellCamp(mapgen.CompoundConfig{W: w, H: h, Tech: tech, Seed: site.Seed})
	guards := min(max(c.ThreatPoints(*q)/rescueGuardPoints, 2), len(camp.Posts))
	weapons := make([]item.Kind, guards)
	for i := range weapons {
		weapons[i] = arms[c.Intn(len(arms))]
	}
	n := b.Camp(camp, q.Target, weapons, site.Seed)
	id, ok := b.Held(pawn.Seed, 0, cell.Inside.X, cell.Inside.Y)
	if n == 0 || !ok {
		return quest.VerdictFail(qd.SiteLost)
	}
	site.Map, site.Count, pawn.Pawn = s.Map, n, uint64(id)
	return quest.VerdictNone
}

// PartDue: the rescue has no part timers.
func (Rescue) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict { return quest.VerdictNone }

// Ended takes the camp's claim off the world.
func (Rescue) Ended(c quest.Ctx, q *qd.Quest) {
	if len(q.Parts) > rescueSite {
		c.R.Sites.Remove(q.Parts[rescueSite].Tile, q.ID)
	}
}

// Detail: the camp's tile and captor, the deadline, the prisoner (its seed
// for the window to preview) and how it stands: calling for help until the
// site opens, held in its cell, freed.
func (Rescue) Detail(q qd.Quest) []quest.Line {
	if len(q.Parts) <= rescuePawn {
		return nil
	}
	site, pawn := q.Parts[rescueSite], q.Parts[rescuePawn]
	state := "quest.rescue.calling"
	switch {
	case pawn.Done:
		state = "quest.rescue.freed"
	case pawn.Pawn != 0:
		state = "quest.rescue.held"
	}
	return []quest.Line{
		{Key: "quest.rescue.site", Args: []string{strconv.Itoa(site.Tile), strconv.Itoa(q.Target)}},
		{Key: "quest.rescue.deadline", Args: []string{strconv.FormatUint(q.Deadline, 10)}},
		{Key: "quest.rescue.pawn", Args: []string{strconv.FormatUint(pawn.Seed, 10)}},
		{Key: state},
	}
}
