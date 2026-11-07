package kinds

import (
	"strconv"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
)

// Refugee is the downed refugee at a site (QST-055..059). It accepts itself
// with a 10 day deadline (Def). The site is a land tile 6..25 tiles away;
// when a caravan opens it the person is laid down on the map, hurt, and
// maybe a few armed strangers stand nearby. "Offer help" (the Fulfil
// command) takes the person into the colony and succeeds the quest; their
// death, leaving, the map closing or the deadline fail it.
//
// The pawn is the reward (Rewards[0].Pawn shows its preview), but the
// person on the site is the one who joins, so the kind marks the quest Paid
// when they join: the root's pay then does not walk a second one in.
type Refugee struct{}

// Part indexes in a refugee quest.
const (
	refugeeSite = 0
	refugeePawn = 1
)

// Guards, ours: one per 150 threat points, 2..5, around the person.
const (
	refugeeMinGuards = 2
	refugeeMaxGuards = 5
	refugeeGuardPts  = 150
	// refugeeGuardDx is how far east of the person the guards stand.
	refugeeGuardDx = 7
)

// Def is the kind's rules.
func (Refugee) Def() qd.Def { return qd.DefOf(qd.Refugee) }

// CanOffer: quest threats are on (the kind is violent) and the colony is
// small enough for a new pawn (the joiner curve).
func (Refugee) CanOffer(c quest.Ctx) bool {
	w := c.R.World
	return w.QuestThreats() && qd.JoinerWeight(w.Colonists()) > 0
}

// Build picks the site tile, the person's seed and, when there is a hostile
// faction, the guards' faction (Target) and their number (site Count).
func (Refugee) Build(c quest.Ctx, q *qd.Quest) bool {
	tile, ok := c.R.World.NearTile(c.Seed(), 6, 25)
	if !ok {
		return false
	}
	var enemies []int
	for _, f := range c.R.World.Factions() {
		if f.Relation == faction.Hostile && f.Human && !f.Defeated {
			enemies = append(enemies, f.Index)
		}
	}
	q.Giver, q.Target = 0, 0
	site := qd.Part{Kind: qd.PartSite, Tile: tile}
	if len(enemies) > 0 {
		q.Target = enemies[c.Intn(len(enemies))] + 1
		site.Count = min(max(c.ThreatPoints(*q)/refugeeGuardPts, refugeeMinGuards), refugeeMaxGuards)
	}
	seed := c.Seed()
	q.Rewards = []qd.Reward{{Pawn: seed}}
	q.Parts = []qd.Part{site, {Kind: qd.PartPawn, Tile: -1, Seed: seed}}
	return true
}

// Accepted: nothing yet; the site is laid when a caravan opens it.
func (Refugee) Accepted(quest.Ctx, *qd.Quest) {}

// Signal reacts to the site and the person.
func (k Refugee) Signal(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	if len(q.Parts) <= refugeePawn || q.Parts[refugeePawn].Kind != qd.PartPawn {
		return quest.VerdictNone
	}
	p := &q.Parts[refugeePawn]
	switch s.Kind {
	case quest.SigSiteOpened:
		return k.open(c, q, s)
	case quest.SigPawnDied, quest.SigPawnLeft:
		if p.Pawn != 0 && uint64(s.Pawn) == p.Pawn {
			return quest.VerdictFail(qd.PawnLost)
		}
	case quest.SigPawnJoined:
		if p.Pawn != 0 && uint64(s.Pawn) == p.Pawn {
			return refugeeJoined(q)
		}
	case quest.SigFulfil:
		id := entity.EntityID(p.Pawn)
		if p.Pawn != 0 && c.R.Pawns.Alive(id) && c.R.Pawns.Join(id) {
			return refugeeJoined(q)
		}
	case quest.SigMapClosed:
		if s.Map != 0 && s.Map == q.Parts[refugeeSite].Map {
			return quest.VerdictFail(qd.SiteLost)
		}
	case quest.SigSiteLost:
		return quest.VerdictFail(qd.SiteLost)
	}
	return quest.VerdictNone
}

// refugeeJoined marks the person joined and the reward paid; success.
func refugeeJoined(q *qd.Quest) quest.Verdict {
	q.Parts[refugeePawn].Done = true
	q.Paid = true
	return quest.VerdictSucceed
}

// open lays the site on the map that just opened: the guards first, then the
// person near the middle. A person who cannot be placed fails the quest as
// SiteLost (there is nobody to help).
func (Refugee) open(c quest.Ctx, q *qd.Quest, s quest.Signal) quest.Verdict {
	site, p := &q.Parts[refugeeSite], &q.Parts[refugeePawn]
	if p.Pawn != 0 || s.Map == 0 {
		return quest.VerdictNone
	}
	b := c.R.Builder
	w, h := b.Size()
	cx, cy := w/2, h/2
	if site.Count > 0 && q.Target != 0 {
		arms := make([]item.Kind, site.Count)
		for i := range arms {
			arms[i] = []item.Kind{item.Club, item.Knife, item.Spear}[c.Intn(3)]
		}
		gx := cx + refugeeGuardDx
		b.Camp(mapgen.Compound{
			X0: gx, Y0: cy, X1: gx, Y1: cy, Centre: mapgen.Point{X: gx, Y: cy},
			Posts: []mapgen.Point{{X: gx, Y: cy - 2}, {X: gx, Y: cy + 2}},
		}, q.Target, arms, c.Seed())
	}
	id, ok := b.Downed(p.Seed, 0, cx, cy)
	if !ok {
		return quest.VerdictFail(qd.SiteLost)
	}
	site.Map, p.Pawn = s.Map, uint64(id)
	return quest.VerdictNone
}

// PartDue: the parts have no timers.
func (Refugee) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict { return quest.VerdictNone }

// Ended: nothing to clean up; the map's own pack-up takes over.
func (Refugee) Ended(quest.Ctx, *qd.Quest) {}

// Detail: the site, the deadline, the person and how things stand.
func (Refugee) Detail(q qd.Quest) []quest.Line {
	if len(q.Parts) <= refugeePawn {
		return nil
	}
	site, p := q.Parts[refugeeSite], q.Parts[refugeePawn]
	pv := quest.PreviewOf(p.Seed)
	lines := []quest.Line{
		{Key: "quest.refugee.site", Args: []string{strconv.Itoa(site.Tile)}},
		{Key: "quest.refugee.deadline", Args: []string{strconv.FormatUint(q.Deadline, 10)}},
		{Key: "quest.refugee.pawn", Args: []string{pv.Name, strconv.Itoa(pv.Age)}},
	}
	switch {
	case q.State == qd.Succeeded:
		lines = append(lines, quest.Line{Key: "quest.refugee.helped"})
	case q.State == qd.Active:
		lines = append(lines, quest.Line{Key: "quest.refugee.waiting"})
		if site.Count > 0 && p.Pawn == 0 {
			lines = append(lines, quest.Line{Key: "quest.refugee.guards"})
		}
	}
	return lines
}
