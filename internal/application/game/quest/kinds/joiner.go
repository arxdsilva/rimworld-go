package kinds

import (
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
)

// Joiner is the threatened joiner (QST-040..044): a pawn the giver's
// enemies are after asks to join. The offer holds one reward, the pawn,
// who walks in at accept (RewardAt OnAccept); 1-6 hours later the threat
// (a raid, a manhunter pack or a forced condition) arrives and the quest
// succeeds. The threat is the cost of the pawn, never a fail condition.
type Joiner struct{}

// Part indexes in a joiner quest.
const (
	joinerPawn   = 0
	joinerThreat = 1
)

// Threat kinds kept in Part.Threat (saved: append-only).
const (
	joinerRaid uint8 = iota + 1
	joinerManhunters
	joinerHeat
	joinerCold
	joinerVolcanic
)

// Threat timing, ours: the threat comes 1..6 hours after accept; a forced
// condition lasts 2..5 days.
const (
	joinerMinHours = 1
	joinerHours    = 6
	joinerMinDays  = 2
	joinerDays     = 4
)

func (Joiner) Def() qd.Def { return qd.DefOf(qd.Joiner) }

// CanOffer: quest threats are on (the kind is violent), the colony is
// small enough for the joiner curve and a giver exists.
func (Joiner) CanOffer(c quest.Ctx) bool {
	w := c.R.World
	return w.QuestThreats() && qd.JoinerWeight(w.Colonists()) > 0 &&
		len(joinerGivers(w.Factions())) > 0
}

// joinerGivers are the factions that may ask: not hostile, able to be
// friendly, not defeated. Ascending by index.
func joinerGivers(fs []quest.FactionView) []int {
	var out []int
	for _, f := range fs {
		if f.Relation != faction.Hostile && f.CanBeFriendly && !f.Defeated {
			out = append(out, f.Index)
		}
	}
	return out
}

// joinerEnemies are the hostile, undefeated factions other than skip.
func joinerEnemies(fs []quest.FactionView, skip int) []int {
	var out []int
	for _, f := range fs {
		if f.Relation == faction.Hostile && !f.Defeated && f.Index != skip {
			out = append(out, f.Index)
		}
	}
	return out
}

// Build picks the giver, the enemy (when there is one), the pawn's seed
// and the threat.
func (Joiner) Build(c quest.Ctx, q *qd.Quest) bool {
	fs := c.R.World.Factions()
	givers := joinerGivers(fs)
	if len(givers) == 0 {
		return false
	}
	g := givers[c.Intn(len(givers))]
	q.Giver = g + 1
	var enemy int
	if es := joinerEnemies(fs, g); len(es) > 0 {
		enemy = es[c.Intn(len(es))] + 1
	}
	q.Target = enemy
	pawn := c.Seed()
	// The only option is the pawn; PickRewards would roll items or
	// goodwill for it, so the option is built here.
	q.Rewards = []qd.Reward{{Pawn: pawn}}
	threat := pickJoinerThreat(c, enemy != 0)
	days := joinerMinDays + c.Intn(joinerDays)
	q.Parts = []qd.Part{
		{Kind: qd.PartPawn, Tile: -1, Seed: pawn},
		{Kind: qd.PartThreat, Tile: -1, Threat: threat, Points: c.ThreatPoints(*q),
			Seed: c.Seed(), Count: days},
	}
	return true
}

// pickJoinerThreat rolls the threat; a raid needs an enemy. Raid and
// manhunters weigh 2, each condition 1 (ours).
func pickJoinerThreat(c quest.Ctx, raid bool) uint8 {
	bag := []uint8{joinerManhunters, joinerManhunters, joinerHeat, joinerCold, joinerVolcanic}
	if raid {
		bag = append(bag, joinerRaid, joinerRaid)
	}
	return bag[c.Intn(len(bag))]
}

// Accepted sets the threat's arrival 1..6 hours from now.
func (Joiner) Accepted(c quest.Ctx, q *qd.Quest) {
	if len(q.Parts) <= joinerThreat {
		return
	}
	h := uint64(joinerMinHours + c.Intn(joinerHours))
	q.Parts[joinerThreat].At = c.Now + h*uint64(clock.TicksPerHour)
}

// Signal: a joiner reacts to no signal (the pawn's fate changes nothing,
// QST-043).
func (Joiner) Signal(quest.Ctx, *qd.Quest, quest.Signal) quest.Verdict { return quest.VerdictNone }

// PartDue fires the threat and succeeds the quest, whether or not the
// threat could start.
func (Joiner) PartDue(c quest.Ctx, q *qd.Quest, part int) quest.Verdict {
	if part != joinerThreat || part >= len(q.Parts) || q.Parts[part].Kind != qd.PartThreat {
		return quest.VerdictNone
	}
	p := q.Parts[part]
	t := c.R.Threats
	switch p.Threat {
	case joinerRaid:
		if !joinerEnemyHostile(c, q.Target) || !t.Raid(q.Target-1, p.Points, p.Seed) {
			// The enemy made peace or the raid could not start: a pack
			// of the same strength comes instead.
			t.Manhunters(p.Points, p.Seed)
		}
	case joinerManhunters:
		t.Manhunters(p.Points, p.Seed)
	default:
		t.Condition(joinerCondition(p.Threat), uint64(p.Count)*uint64(clock.TicksPerDay))
	}
	return quest.VerdictSucceed
}

// joinerEnemyHostile: faction ref (1 + index) is still hostile.
func joinerEnemyHostile(c quest.Ctx, ref int) bool {
	return ref > 0 && c.R.Goodwill.Relation(ref-1) == faction.Hostile
}

func joinerCondition(t uint8) uint8 {
	switch t {
	case joinerHeat:
		return quest.CondHeatWave
	case joinerCold:
		return quest.CondColdSnap
	}
	return quest.CondVolcanicWinter
}

// Ended: nothing to clean up (the pawn is a plain colonist).
func (Joiner) Ended(quest.Ctx, *qd.Quest) {}

// Detail names the giver (its ref, 1 + index, for the window to resolve)
// and, vaguely, the trouble to come.
func (Joiner) Detail(q qd.Quest) []quest.Line {
	lines := []quest.Line{{Key: "quest.joiner.about", Args: []quest.Arg{quest.Faction(q.Giver)}}}
	switch {
	case q.State == qd.Offered:
		lines = append(lines, quest.Line{Key: "quest.joiner.warning"})
	case len(q.Parts) > joinerThreat && q.Parts[joinerThreat].Done:
		lines = append(lines, quest.Line{Key: "quest.joiner.arrived"})
	case q.State == qd.Active:
		lines = append(lines, quest.Line{Key: "quest.joiner.coming"})
	}
	return lines
}
