package game

import (
	"fmt"
	"math"
	"slices"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// quest_host.go: the quests root's game glue (QST-1). game/quest owns the
// quests; questHost is every role it needs (quest/deps.go), and the free
// functions hook it into the tick, commands, signals and the read model.
// Only a setup colony has quests (decision 17). Sites, manhunters,
// conditions and ambushes say "not available" until the kinds that need
// them land (QST-2a, QST-2b).

// QuestView is one quest in the read model (game/quest).
type QuestView = quest.View

type questHost struct{ g *Game }

// questOf is the colony's quests root, built on first use (nil: legacy).
func questOf(g *Game) *quest.Quests {
	if !g.hasSetup() {
		return nil
	}
	if w := g.worldS(); w.quests == nil {
		w.quests = quest.New(questHost{g}.roles(), nil) // the kinds join in QST-2
	}
	return g.worldSt.quests
}

func (h questHost) roles() quest.Roles {
	return quest.Roles{World: h, Clock: h, Sites: h, Pawns: h, Items: h, Threats: h, Letters: h, Goodwill: h, Rolls: h}
}

// questTick runs the offer cadence (day) or the due timers (hour).
func questTick(g *Game, now uint64, day bool) {
	if q := questOf(g); q != nil && day {
		q.Daily(now)
	} else if q != nil {
		q.Hourly(now)
	}
}

// questSignal hands a game event to the quests it concerns.
func questSignal(g *Game, s quest.Signal) {
	if g.worldSt != nil && g.worldSt.quests != nil {
		g.worldSt.quests.Signal(s)
	}
}

// questCommand routes the quest commands (command.go: Index the quest id,
// Count the choice, Target the caravan; SetRewardPref: faction ref in
// Index, the setting in Enabled). A refused one says why.
func questCommand(g *Game, c command.Command) {
	q, id := questOf(g), qd.ID(c.Index)
	if q == nil {
		return
	}
	cmd := map[command.Kind]quest.Cmd{command.AcceptQuest: quest.Accept(id, c.Count), command.DismissQuest: quest.Dismiss(id),
		command.RestoreQuest: quest.Restore(id), command.FulfilRequest: quest.Fulfil(id, uint64(c.Target)),
		command.Negotiate: quest.Negotiate(id, uint64(c.Target)), command.SetRewardPref: quest.SetPref(c.Index, c.Enabled)}[c.Kind]
	if err := q.Apply(cmd); err != nil {
		g.worldNote("The quest order was refused: %v.", err)
	}
}

// questViews is the read model's quest list, memoized on the root's version.
func questViews(g *Game) []QuestView {
	w := g.worldSt
	if w == nil || w.quests == nil {
		return nil
	}
	if v := w.quests.Version(); w.questMemo.views == nil || w.questMemo.ver != v {
		w.questMemo.views, w.questMemo.ver = w.quests.View(), v
	}
	return w.questMemo.views
}

// questMemo is the last quest list built and its version.
type questMemo struct {
	ver   uint64
	views []QuestView
}

func (h questHost) Landed() uint64                                 { return h.g.dayOff() }
func (h questHost) Colonists() int                                 { return len(h.g.colonistsHomeAndAway()) }
func (h questHost) Points() int                                    { return h.g.threatPoints() }
func (h questHost) ThreatScalePct() int                            { return h.g.diff().ThreatScale }
func (h questHost) RewardPct() int                                 { return h.g.diff().QuestRewardPct }
func (h questHost) QuestThreats() bool                             { return h.g.diff().QuestThreats }
func (h questHost) Factions() []quest.FactionView                  { return quest.FactionViews(h.g.ng.factions) }
func (h questHost) NearTile(uint64, int, int) (int, bool)          { return 0, false } // QST-2b
func (h questHost) Now() uint64                                    { return uint64(h.g.clock.Now()) }
func (h questHost) Intn(n int) int                                 { return h.g.stream(streamQuest).IntN(n) }
func (h questHost) Seed() uint64                                   { return uint64(h.Intn(math.MaxInt))*0x9e3779b97f4a7c15 ^ 0x51ed }
func (h questHost) Count() int                                     { return len(h.g.ng.factions) }
func (h questHost) Alive(id entity.EntityID) bool                  { return h.g.alive(id) }
func (h questHost) Place(quest.Site) bool                          { return false } // QST-2b
func (h questHost) Remove(int, qd.ID)                              {}               // QST-2b
func (h questHost) Preview(seed uint64) quest.PawnPreview          { return quest.PreviewOf(seed) }
func (h questHost) GrantXP(id entity.EntityID, s skill.ID, xp int) { h.g.learn(id, s, xp) }
func (h questHost) Has(cid uint64, l []trade.Line) bool            { return travelOf(h.g).HasLines(cid, l) }
func (h questHost) Take(cid uint64, l []trade.Line) bool           { return travelOf(h.g).TakeLines(cid, l) }
func (h questHost) Manhunters(int, uint64) bool                    { return false } // QST-2a
func (h questHost) Condition(uint8, uint64) bool                   { return false } // QST-2a
func (h questHost) Ambush(uint64, int, int, uint64) bool           { return false } // QST-2a

func (h questHost) Settlement(f int, seed uint64) (int, bool) {
	return quest.PickSettlement(h.g.ng.factions, f, seed)
}

func (h questHost) WalkIn(seed uint64) (entity.EntityID, bool) {
	g := h.g
	ex, ey, ok := g.worldEdge()
	if x, y, in := g.insetLandNear(ex, ey, nil); ok && in {
		if id, ok := walkInColonist(g, quest.PawnOf(seed), x, y); ok { // visitors.go
			g.addHistory("joined", g.personName(id)+" joined the colony through a quest.", id)
			questSignal(g, quest.Signal{Kind: quest.SigPawnJoined, Pawn: id})
			return id, true
		}
	}
	return 0, false
}

func (h questHost) Join(id entity.EntityID) bool {
	ok := h.g.visitor(id) && h.g.alive(id)
	if ok {
		careGuests{h.g}.join(id) // visitors.go
	}
	return ok
}

// BestSocial is caravan cid's living member with the best Social skill.
func (h questHost) BestSocial(cid uint64) (entity.EntityID, int, bool) {
	best, lvl := entity.EntityID(0), -1
	if i := slices.IndexFunc(h.g.worldS().caravans, func(c maps.Caravan) bool { return c.C.ID == cid }); i >= 0 {
		for _, m := range h.g.worldSt.caravans[i].C.Members {
			if l := int(h.g.skillOf(entity.EntityID(m), skill.Social).Level); h.g.alive(entity.EntityID(m)) && l > lvl {
				best, lvl = entity.EntityID(m), l
			}
		}
	}
	return best, lvl, lvl >= 0
}

func (h questHost) Pod(lines []trade.Line, _ uint64) bool {
	h.g.sendTradePod(slices.Clone(lines), h.Now()) // trade_orbital.go
	return len(lines) > 0
}

func (h questHost) IntoCaravan(cid uint64, lines []trade.Line) bool {
	ok := slices.ContainsFunc(h.g.worldS().caravans, func(c maps.Caravan) bool { return c.C.ID == cid })
	if ok {
		travelOf(h.g).CarryLines(cid, lines)
	}
	return ok
}

// Raid sends faction f at the colony with points, armed by its tech.
func (h questHost) Raid(f, points int, _ uint64) bool {
	g := h.g
	ok := f >= 0 && f < len(g.ng.factions) && points > 0
	if ok {
		kit := g.raidKitOf(points, faction.DefOf(g.ng.factions[f].Type).Tech)
		g.raidFrom(kit, g.raid.Threats.Plan(g.relDay(h.Now())), f+1)
	}
	return ok
}

func (h questHost) Post(l quest.Letter) {
	name := quest.KindName(l.QuestKind)
	switch l.Kind {
	case quest.LetterOffer:
		letterf(h.g, letters.Info, "Quest: "+name, 0, "A new quest: %s. Open the Quests window to read it.", name)
	case quest.LetterFailed:
		letterf(h.g, letters.Caution, "Quest failed: "+name, 0, "The quest %s has failed: %s.", name, quest.ReasonText(l.Reason))
	case quest.LetterSucceeded:
		letterf(h.g, letters.Good, "Quest done: "+name, 0, "The quest %s is done.", name)
	default:
		return
	}
	h.g.letters.EditLast(func(x *letters.Letter) { x.Quest = uint32(l.Quest) })
}

func (h questHost) Adjust(f, delta int) {
	if f >= 0 && f < len(h.g.ng.factions) {
		faction.Adjust(&h.g.ng.factions[f], delta, fmt.Sprintf("Day %d: a quest (%+d goodwill)", h.g.relDay(h.Now())+1, delta))
	}
}

func (h questHost) Relation(f int) faction.Relation {
	if f < 0 || f >= len(h.g.ng.factions) {
		return faction.Neutral
	}
	return h.g.ng.factions[f].Relation
}
