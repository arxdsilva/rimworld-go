package game

import (
	"fmt"
	"math"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"slices"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/quest/kinds"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
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
		w.quests = quest.New(questHost{g}.roles(), kinds.All())
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

func (h questHost) Landed() uint64                        { return h.g.dayOff() }
func (h questHost) Colonists() int                        { return len(h.g.colonistsHomeAndAway()) }
func (h questHost) ThreatScalePct() int                   { return h.g.diff().ThreatScale }
func (h questHost) RewardPct() int                        { return h.g.diff().QuestRewardPct }
func (h questHost) QuestThreats() bool                    { return h.g.diff().QuestThreats }
func (h questHost) Factions() []quest.FactionView         { return quest.FactionViews(h.g.ng.factions) }
func (h questHost) Now() uint64                           { return uint64(h.g.clock.Now()) }
func (h questHost) Intn(n int) int                        { return h.g.stream(streamQuest).IntN(n) }
func (h questHost) Seed() uint64                          { return uint64(h.Intn(math.MaxInt))*0x9e3779b97f4a7c15 ^ 0x51ed }
func (h questHost) Count() int                            { return len(h.g.ng.factions) }
func (h questHost) Alive(id entity.EntityID) bool         { return h.g.alive(id) }
func (h questHost) Preview(seed uint64) quest.PawnPreview { return quest.PreviewOf(seed) }

// GrantXP is a fixed award: no learn rate, passion or daily cap (QST-076).
func (h questHost) GrantXP(id entity.EntityID, s skill.ID, xp int) {
	set := h.g.sk.skills[id]
	if set == nil || s >= skill.NumSkills || h.g.incapableOf(id).DisablesSkill(s) {
		return
	}
	lv := set[s].Level
	if skill.Grant(&set[s], xp); set[s].Level != lv {
		h.g.rebuildStats(id)
	}
}
func (h questHost) Has(cid uint64, l []trade.Line) bool  { return travelOf(h.g).HasLines(cid, l) }
func (h questHost) Take(cid uint64, l []trade.Line) bool { return travelOf(h.g).TakeLines(cid, l) }

func (h questHost) Deposit(t int) (trade.Line, bool) {
	k, n, ok := travelOf(h.g).Deposit(t)
	return trade.Line{Kind: k, Qty: n}, ok
}

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

// CaravanAt is caravan cid's tile while it stands (arrived or visiting).
func (h questHost) CaravanAt(cid uint64) (int, bool) {
	i := slices.IndexFunc(h.g.worldS().caravans, func(c maps.Caravan) bool { return c.C.ID == cid })
	if i < 0 || h.g.worldSt.caravans[i].C.State != caravan.Arrived && h.g.worldSt.caravans[i].C.State != caravan.Visiting {
		return 0, false
	}
	return h.g.worldSt.caravans[i].C.Tile, true
}

func (h questHost) IntoCaravan(cid uint64, lines []trade.Line) bool {
	ok := slices.ContainsFunc(h.g.worldS().caravans, func(c maps.Caravan) bool { return c.C.ID == cid })
	if ok {
		travelOf(h.g).CarryLines(cid, lines)
	}
	return ok
}

// Points are the raid points of the colony with every caravan's people
// and goods added (QST-025): one difficulty.ThreatPoints call.
func (h questHost) Points() int {
	g := h.g
	wealth, people := g.Wealth(), g.standingColonists()
	for _, c := range g.worldS().caravans {
		people += len(c.C.Members)
		for _, st := range travelOf(g).Goods(c.C.ID) {
			wealth += item.StackValue(st)
		}
	}
	years := float64(g.rel(h.Now())) / float64(clock.TicksPerYear)
	return difficulty.ThreatPoints(g.ng.setup.Difficulty, wealth, people, years, int(g.ng.adapt.Value))
}

// NearTile is a free land tile lo..hi hops from home (QST-117).
func (h questHost) NearTile(seed uint64, lo, hi int) (int, bool) {
	return travelOf(h.g).FreeTile(seed, lo, hi, func(t int) bool { return questTileTaken(h.g, t, 0) })
}

// Place claims s.Tile for its quest: a quest site is the quest's own part
// (the read model shows it), so placing only checks the tile is free.
// Site maps join in QST-2b.
func (h questHost) Place(s quest.Site) bool {
	_, settled := quest.SettlementAt(h.g.ng.factions, s.Tile)
	return s.Tile >= 0 && !settled && !questTileTaken(h.g, s.Tile, s.Quest)
}

func (h questHost) Remove(int, qd.ID) {}

// questTileTaken: world tile t holds a map or another live quest's site.
func questTileTaken(g *Game, t int, self qd.ID) bool {
	return t == g.colonyTile() || slices.ContainsFunc(g.worldS().atlas.Maps, func(m maps.Meta) bool { return m.Tile == t }) ||
		slices.ContainsFunc(questViews(g), func(v QuestView) bool { return v.ID != self && !v.State.Terminal() && v.Tile == t })
}

// Manhunters sends a pack of about one animal per 100 points (3..12, ours).
func (h questHost) Manhunters(points int, _ uint64) bool {
	h.g.manhunterPack(min(max(points/100, 3), 12)) // handling.go
	return true
}

// Condition forces a climate event for ticks (QST-042), with its letter.
func (h questHost) Condition(k uint8, ticks uint64) bool {
	id := map[uint8]incident.ID{quest.CondHeatWave: incident.HeatWave, quest.CondColdSnap: incident.ColdSnap, quest.CondVolcanicWinter: incident.VolcanicWinter}[k]
	ok := id != 0 && h.g.incidents.StartClimate(id, h.Now(), ticks)
	if title, text := incident.ClimateLetter(id, ticks); ok {
		hx, hy := h.g.home()
		h.g.post(MessageView{Tick: h.Now(), X: hx, Y: hy, Letter: true, Title: title, Kind: letters.Caution, Text: text})
	}
	return ok
}

// Ambush has faction f attack caravan cid where it stands (QST-079); the
// encounter map sizes its own raiders.
func (h questHost) Ambush(cid uint64, f, _ int, _ uint64) bool {
	return maps.AmbushCaravan(fleetOf(h.g), entity.EntityID(cid), f+1, h.Now())
}

// ScannerFound offers the ore-lump quest for the scanner's new site on t
// (QST-116); travel's Quester role.
func (h travelHost) ScannerFound(t int) bool {
	q := questOf(h.g)
	if q == nil {
		return false
	}
	_, ok := q.OfferAt(qd.OreLump, t)
	return ok
}

// mapsQuests is game/maps' Quester role (QST-074, QST-118).
type mapsQuests struct{ g *Game }

func (h mapsQuests) CaravanArrived(cid uint64, t int) {
	questSignal(h.g, quest.Signal{Kind: quest.SigCaravanArrived, Tile: t, Caravan: cid})
}

// HoldsVisit: an active trade request waits at tile t.
func (h mapsQuests) HoldsVisit(t int) bool {
	return slices.ContainsFunc(questViews(h.g), func(v QuestView) bool { return v.State == qd.Active && v.Kind == qd.Request && v.Tile == t })
}

// SettlementLost: the quests at tile t lose their site; a defeated faction's
// quests learn it too (talks, a request).
func (h mapsQuests) SettlementLost(fi, t int, defeated bool) {
	questSignal(h.g, quest.Signal{Kind: quest.SigSiteLost, Tile: t})
	if defeated {
		questSignal(h.g, quest.Signal{Kind: quest.SigFactionDefeated, Faction: fi + 1})
	}
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
