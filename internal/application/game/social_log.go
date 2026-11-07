package game

import (
	"rimworld-go/internal/application/game/sociallog"
	"rimworld-go/internal/domain/sim/entity"
)

// The social log (ENG-003, M3/M4): a per-pawn ring of the interactions it
// took part in, separate from the combat log. Entries keep ids, a kind and
// a topic; the prose is built when the log is read, so logging allocates
// nothing and a renamed pawn reads right.

// socialLogCap is the per-pawn ring size.
const socialLogCap = 30

// socialLogKind is a logged social event (saved, append only). The first
// values mirror social.Interaction.
type socialLogKind uint8

const (
	logChitchat       socialLogKind = sociallog.Chitchat
	logDeepTalk       socialLogKind = sociallog.DeepTalk
	logSlight         socialLogKind = sociallog.Slight
	logInsult         socialLogKind = sociallog.Insult
	logKindWords      socialLogKind = sociallog.KindWords
	logRomance        socialLogKind = sociallog.Romance
	logBreakup        socialLogKind = sociallog.Breakup
	logProposal       socialLogKind = sociallog.Proposal
	numSocialLogKinds socialLogKind = sociallog.Kinds
)

// socialLogEntry is one line: A did Kind to (or with) B about Topic.
type socialLogEntry struct {
	Tick  uint64
	Kind  socialLogKind
	A, B  entity.EntityID
	Topic uint8
}

// addSocialLog records the event on both participants' rings.
func (g *Game) addSocialLog(a, b entity.EntityID, k socialLogKind, topic uint8, now uint64) {
	e := socialLogEntry{Tick: now, Kind: k, A: a, B: b, Topic: topic}
	s := g.socState()
	for _, id := range [2]entity.EntityID{a, b} {
		if id == 0 {
			continue
		}
		l := s.logs[id]
		if len(l) < socialLogCap {
			l = append(l, e)
		} else {
			copy(l, l[1:])
			l[len(l)-1] = e
		}
		s.logs[id] = l
	}
}

// socialLogText is an entry's prose (game/sociallog).
func (g *Game) socialLogText(e socialLogEntry) string {
	return sociallog.Text(uint8(e.Kind), e.Topic, g.personName(e.A), g.personName(e.B))
}

// socialLogViews is id's social log, newest first.
func (g *Game) socialLogViews(id entity.EntityID) []SocialLogLineView {
	if g.soc == nil {
		return nil
	}
	l := g.soc.logs[id]
	out := make([]SocialLogLineView, 0, len(l))
	for i := len(l) - 1; i >= 0; i-- {
		e := l[i]
		icon := ""
		if e.Kind < numSocialLogKinds {
			icon = "social_" + sociallog.Icon(uint8(e.Kind))
		}
		out = append(out, SocialLogLineView{Tick: e.Tick, Text: g.socialLogText(e), Icon: icon, Weighty: sociallog.Weighty(uint8(e.Kind))})
	}
	return out
}

// lastInteraction is id's latest social event as a bubble kind and tick.
func (g *Game) lastInteraction(id entity.EntityID) (string, uint64) {
	if g.soc == nil {
		return "", 0
	}
	l := g.soc.logs[id]
	if len(l) == 0 {
		return "", 0
	}
	e := l[len(l)-1]
	if e.Kind >= numSocialLogKinds {
		return "", 0
	}
	return sociallog.Icon(uint8(e.Kind)), e.Tick
}
