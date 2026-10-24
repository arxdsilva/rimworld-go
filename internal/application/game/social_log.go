package game

import (
	"fmt"

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
	logChitchat socialLogKind = iota
	logDeepTalk
	logSlight
	logInsult
	logKindWords
	logRomance
	logBreakup
	logProposal
	numSocialLogKinds
)

// socialLogEntry is one line: A did Kind to (or with) B about Topic.
type socialLogEntry struct {
	Tick  uint64
	Kind  socialLogKind
	A, B  entity.EntityID
	Topic uint8
}

// socialIcon is each kind's bubble and log icon suffix (ui_icon/social_*).
var socialIcon = [numSocialLogKinds]string{
	"chitchat", "deep_talk", "slight", "insult", "kind_words", "romance", "breakup", "proposal",
}

// weightyLog marks the kinds with consequences (tinted in the log).
var weightyLog = [numSocialLogKinds]bool{logSlight: true, logInsult: true, logRomance: true, logBreakup: true, logProposal: true}

// Topics (original prose) per interaction kind.
var (
	chatTopics = []string{
		"the weather", "an old song", "the harvest", "a strange dream", "far-off cities", "childhood games",
		"favourite tools", "the night sky", "the best way to cook rice", "wild animals", "a rumour from the road",
		"the colony's walls", "rock and ore", "the price of steel", "a funny accident", "home cooking",
	}
	deepTopics = []string{
		"what makes a place home", "what comes after death", "old regrets", "the nature of courage",
		"what they would change about the past", "faith and doubt", "the people they left behind", "why we keep going",
	}
	slightTopics = []string{"cooking", "haircut", "taste in clothes", "work", "laugh", "walk", "snoring", "table manners"}
	insultTopics = []string{"cooking", "looks", "intelligence", "family", "work ethic", "voice", "hygiene", "past"}
	kindTopics   = []string{"patience", "hard work", "smile", "skill", "cooking", "courage", "kindness", "good humour"}
)

func topicsOf(k socialLogKind) []string {
	switch k {
	case logChitchat:
		return chatTopics
	case logDeepTalk:
		return deepTopics
	case logSlight:
		return slightTopics
	case logInsult:
		return insultTopics
	case logKindWords:
		return kindTopics
	}
	return nil
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

// socialLogText is an entry's prose.
func (g *Game) socialLogText(e socialLogEntry) string {
	a, b := g.personName(e.A), g.personName(e.B)
	topic := ""
	if ts := topicsOf(e.Kind); len(ts) > 0 {
		topic = ts[int(e.Topic)%len(ts)]
	}
	switch e.Kind {
	case logChitchat:
		return fmt.Sprintf("%s and %s chatted about %s.", a, b, topic)
	case logDeepTalk:
		return fmt.Sprintf("%s and %s talked at length about %s.", a, b, topic)
	case logSlight:
		return fmt.Sprintf("%s made a snide remark about %s's %s.", a, b, topic)
	case logInsult:
		return fmt.Sprintf("%s insulted %s's %s.", a, b, topic)
	case logKindWords:
		return fmt.Sprintf("%s praised %s's %s.", a, b, topic)
	case logRomance:
		return fmt.Sprintf("%s tried to win %s's heart.", a, b)
	case logBreakup:
		return fmt.Sprintf("%s ended things with %s.", a, b)
	case logProposal:
		return fmt.Sprintf("%s asked %s to marry.", a, b)
	}
	return fmt.Sprintf("%s and %s spent time together.", a, b)
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
			icon = "social_" + socialIcon[e.Kind]
		}
		out = append(out, SocialLogLineView{Tick: e.Tick, Text: g.socialLogText(e), Icon: icon, Weighty: e.Kind < numSocialLogKinds && weightyLog[e.Kind]})
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
	return socialIcon[e.Kind], e.Tick
}
