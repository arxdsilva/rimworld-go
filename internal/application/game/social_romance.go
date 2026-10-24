package game

import (
	"fmt"

	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Romance (ENG-003, M5): couples move through the domain's pair machine
// (social.Pair) on their shared edge. Attempts, breakups and proposals
// are interaction slots chosen by social.Choose; this file resolves them
// with the social stream and leaves the memories, moods, letters and
// history lines. Ex-partners can reconcile, but not within
// social.BreakupGrace of the breakup (the reconcile cooldown), and no
// couple breaks up within BreakupGrace of its last change: neither
// direction can flap. A breakup calls off the couple's wedding
// (gathering.go gatherAbort).

// pairRank orders a pawn's partners: spouse, fiancé, lover.
func pairRank(k social.PairKind) int {
	switch k {
	case social.PairSpouse:
		return 3
	case social.PairFiance:
		return 2
	case social.PairLover:
		return 1
	}
	return 0
}

// partners lists id's living partners (lover, fiancé, spouse), strongest
// first, then by id.
func (g *Game) partners(id entity.EntityID) []entity.EntityID {
	if g.soc == nil {
		return nil
	}
	var out []entity.EntityID
	for _, o := range g.soc.adj[id] {
		e := g.soc.edges[socKey(id, o)]
		if e == nil || !e.pair.Kind.Active() {
			continue
		}
		if p := g.personOf(o); p != nil && p.Where == whereDead {
			continue
		}
		out = append(out, o)
	}
	for i := 1; i < len(out); i++ { // insertion sort: a handful at most
		for j := i; j > 0 && pairRank(g.pairOf(id, out[j]).Kind) > pairRank(g.pairOf(id, out[j-1]).Kind); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// partnerOf is id's main partner (0: none).
func (g *Game) partnerOf(id entity.EntityID) entity.EntityID {
	if ps := g.partners(id); len(ps) > 0 {
		return ps[0]
	}
	return 0
}

// partneredExcept reports id has a partner other than other.
func (g *Game) partneredExcept(id, other entity.EntityID) bool {
	for _, p := range g.partners(id) {
		if p != other {
			return true
		}
	}
	return false
}

// engagedElsewhere reports id is engaged to or married to someone other
// than other (no second proposal, no second wedding).
func (g *Game) engagedElsewhere(id, other entity.EntityID) bool {
	for _, p := range g.partners(id) {
		if k := g.pairOf(id, p).Kind; p != other && (k == social.PairFiance || k == social.PairSpouse) {
			return true
		}
	}
	return false
}

// bloodKin reports a and b are family by blood (romance is out).
func (g *Game) bloodKin(a, b entity.EntityID) bool {
	for _, k := range social.KinOf(social.ID(a), social.ID(b), famGraph{g}) {
		if k < social.StepParent {
			return true
		}
	}
	return false
}

// romanceable reports id is a colonist who may court and be courted.
func (g *Game) romanceable(id entity.EntityID) bool {
	p := g.personOf(id)
	return p != nil && p.Where == whereColony && !g.isRaider(id)
}

// attractionOf is a's attraction to b, permille (0: ineligible).
func (g *Game) attractionOf(a, b entity.EntityID) int32 {
	if !g.romanceable(a) || !g.romanceable(b) {
		return 0
	}
	return social.Attraction(g.socialPerson(a), g.socialPerson(b), g.bloodKin(a, b), g.opinion(a, b))
}

// romanceAttraction is the Attraction an interaction context carries for
// initiator a toward b: zero while they are a couple or a rebuff cooldown
// runs, so no attempt is chosen then.
func (g *Game) romanceAttraction(a, b entity.EntityID, now uint64) int32 {
	p := g.pairOf(a, b)
	if p.Kind.Active() || !p.CanRomance(now) {
		return 0
	}
	return g.attractionOf(a, b)
}

// romanceInteraction handles a romance, breakup or proposal interaction
// a started with b; false lets the pawn chitchat instead.
func (g *Game) romanceInteraction(k social.Interaction, a, b entity.EntityID) bool {
	if a == b || a == 0 || b == 0 {
		return false
	}
	now := uint64(g.clock.Now())
	switch k {
	case social.RomanceAttempt:
		return g.tryRomance(a, b, now)
	case social.Breakup:
		return g.breakUp(a, b, now)
	case social.Proposal:
		return g.propose(a, b, now)
	}
	return false
}

// tryRomance resolves a's romance attempt on b: a new couple (an affair
// when either was partnered), or a rebuff with its memories and cooldown.
func (g *Game) tryRomance(a, b entity.EntityID, now uint64) bool {
	if g.romanceAttraction(a, b, now) <= 0 {
		return false
	}
	p := g.pairOf(a, b)
	chance := social.RomanceSuccess(g.attractionOf(b, a), g.partneredExcept(b, a))
	g.addSocialLog(a, b, logRomance, 0, now)
	if int32(g.stream(streamSocial).IntN(1000)) >= chance {
		g.addOpinionMemory(a, b, social.MemRebuffedMe, now)
		g.addOpinionMemory(b, a, social.MemFailedRomance, now)
		g.memories[a] = g.memories[a].AddFor(mood.RebuffedMood, uint64(b), now)
		g.memories[b] = g.memories[b].AddFor(mood.HadToRebuff, uint64(a), now)
		p.Rebuff(now)
		g.setPair(a, b, p)
		return true
	}
	cheatedA, cheatedB := g.partners(a), g.partners(b)
	if !p.Transition(social.PairLover, now) {
		return false
	}
	g.setPair(a, b, p)
	text := fmt.Sprintf("%s and %s are now lovers.", g.personName(a), g.personName(b))
	g.romanceLetter("New lovers", text, a)
	g.addHistory("romance", text, a, b)
	for _, c := range cheatedA {
		g.cheatedOn(c, a, b, now)
	}
	for _, c := range cheatedB {
		g.cheatedOn(c, b, a, now)
	}
	return true
}

// cheatedOn: victim learns its partner cheater took up with other.
func (g *Game) cheatedOn(victim, cheater, other entity.EntityID, now uint64) {
	if victim == other {
		return
	}
	g.addOpinionMemory(victim, cheater, social.MemCheatedOnMe, now)
	g.memories[victim] = g.memories[victim].AddFor(mood.CheatedOn, uint64(cheater), now)
	g.romanceLetter("Affair", fmt.Sprintf("%s has been cheating on %s with %s.",
		g.personName(cheater), g.personName(victim), g.personName(other)), victim)
}

// breakUp ends a's partnership with b: ex-lovers, or divorce for a married
// couple. The dumped partner remembers it; the pair cannot reconcile for
// BreakupGrace.
func (g *Game) breakUp(a, b entity.EntityID, now uint64) bool {
	p := g.pairOf(a, b)
	if !p.CanBreakUp(now) {
		return false
	}
	divorce := p.Kind == social.PairSpouse
	to, mem, md := social.PairExLover, social.MemBrokeUpWithMe, mood.BrokenUpWith
	if divorce {
		to, mem, md = social.PairExSpouse, social.MemDivorcedMe, mood.Divorced
	}
	if !p.Transition(to, now) {
		return false
	}
	p.NextRomanceAt = max(p.NextRomanceAt, now+social.BreakupGrace) // reconcile cooldown
	g.setPair(a, b, p)
	g.addOpinionMemory(b, a, mem, now)
	g.memories[b] = g.memories[b].AddFor(md, uint64(a), now)
	g.addSocialLog(a, b, logBreakup, 0, now)
	na, nb := g.personName(a), g.personName(b)
	if divorce {
		text := fmt.Sprintf("%s and %s have divorced.", na, nb)
		g.romanceLetter("Divorce", text, a)
		g.addHistory("divorce", text, a, b)
	} else {
		text := fmt.Sprintf("%s broke up with %s.", na, nb)
		g.romanceLetter("Breakup", text, a)
		g.addHistory("breakup", text, a, b)
	}
	return true
}

// propose resolves a's marriage proposal to b: engaged, or turned down
// with the memories and the proposal cooldown.
func (g *Game) propose(a, b entity.EntityID, now uint64) bool {
	p := g.pairOf(a, b)
	if !p.CanPropose(now) || g.engagedElsewhere(a, b) || g.engagedElsewhere(b, a) {
		return false
	}
	accept := int32(g.stream(streamSocial).IntN(1000)) < social.ProposalAcceptFor(g.opinion(b, a), now-min(p.Since, now))
	g.addSocialLog(a, b, logProposal, 0, now)
	if !accept {
		p.RejectProposal(now)
		g.setPair(a, b, p)
		g.addOpinionMemory(a, b, social.MemRejectedMyProposal, now)
		g.addOpinionMemory(b, a, social.MemIRejectedProposal, now)
		g.memories[a] = g.memories[a].AddFor(mood.ProposalRejected, uint64(b), now)
		g.romanceLetter("Proposal rejected", fmt.Sprintf("%s proposed to %s and was turned down.",
			g.personName(a), g.personName(b)), a)
		return true
	}
	if !p.Transition(social.PairFiance, now) {
		return false
	}
	g.setPair(a, b, p)
	text := fmt.Sprintf("%s and %s are engaged to be married.", g.personName(a), g.personName(b))
	g.romanceLetter("Engagement", text, a)
	g.addHistory("engagement", text, a, b)
	return true
}

// romanceLetter posts a letter about pawn focus.
func (g *Game) romanceLetter(title, text string, focus entity.EntityID) {
	g.pawnLetter(title, text, focus)
}

// pawnLetter posts a letter titled title about creature focus (shown once,
// the jump link follows it).
func (g *Game) pawnLetter(title, text string, focus entity.EntityID) {
	pos, _ := g.positions.Get(focus)
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: text, X: pos.X, Y: pos.Y, Focus: focus, Letter: true, Title: title})
}

// romanceNote is the Social tab's reason id and o can't become a couple
// ("" when they could, or already are one).
func (g *Game) romanceNote(id, o entity.EntityID) string {
	if g.pairOf(id, o).Kind.Active() {
		return ""
	}
	if !g.romanceable(id) || !g.romanceable(o) {
		return ""
	}
	pi, po := g.socialPerson(id), g.socialPerson(o)
	switch {
	case pi.Dead || po.Dead:
		return ""
	case g.bloodKin(id, o):
		return "family"
	case pi.AgeYears < social.MinRomanceAge || po.AgeYears < social.MinRomanceAge:
		return "too young"
	case g.partneredExcept(id, o):
		return "already partnered"
	case g.attractionOf(id, o) <= 0:
		return "not attracted"
	}
	return ""
}
