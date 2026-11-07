// Package sociallog is the social log's words (ENG-003, M3/M4): each
// logged kind's icon, weight, topics and prose, moved out of the game
// package, which keeps the per-pawn rings (social_log.go) and builds a
// line's text here when the log is read. It is pure: no state, no host.
package sociallog

import "fmt"

// Logged social event kinds (the game's socialLogKind; saved, append
// only). The first values mirror social.Interaction.
const (
	Chitchat = iota
	DeepTalk
	Slight
	Insult
	KindWords
	Romance
	Breakup
	Proposal
	Kinds // the number of kinds
)

// icon is each kind's bubble and log icon suffix (ui_icon/social_*).
var icon = [Kinds]string{
	"chitchat", "deep_talk", "slight", "insult", "kind_words", "romance", "breakup", "proposal",
}

// weighty marks the kinds with consequences (tinted in the log).
var weighty = [Kinds]bool{Slight: true, Insult: true, Romance: true, Breakup: true, Proposal: true}

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

func topicsOf(k uint8) []string {
	switch k {
	case Chitchat:
		return chatTopics
	case DeepTalk:
		return deepTopics
	case Slight:
		return slightTopics
	case Insult:
		return insultTopics
	case KindWords:
		return kindTopics
	}
	return nil
}

// Topics is how many topics kind k picks from (0: it has none).
func Topics(k uint8) int { return len(topicsOf(k)) }

// Icon is kind k's icon suffix ("" for an unknown kind).
func Icon(k uint8) string {
	if k >= Kinds {
		return ""
	}
	return icon[k]
}

// Weighty reports a kind with consequences.
func Weighty(k uint8) bool { return k < Kinds && weighty[k] }

// Text is the prose of a kind k line between a and b (names) about topic.
func Text(k, topic uint8, a, b string) string {
	t := ""
	if ts := topicsOf(k); len(ts) > 0 {
		t = ts[int(topic)%len(ts)]
	}
	switch k {
	case Chitchat:
		return fmt.Sprintf("%s and %s chatted about %s.", a, b, t)
	case DeepTalk:
		return fmt.Sprintf("%s and %s talked at length about %s.", a, b, t)
	case Slight:
		return fmt.Sprintf("%s made a snide remark about %s's %s.", a, b, t)
	case Insult:
		return fmt.Sprintf("%s insulted %s's %s.", a, b, t)
	case KindWords:
		return fmt.Sprintf("%s praised %s's %s.", a, b, t)
	case Romance:
		return fmt.Sprintf("%s tried to win %s's heart.", a, b)
	case Breakup:
		return fmt.Sprintf("%s ended things with %s.", a, b)
	case Proposal:
		return fmt.Sprintf("%s asked %s to marry.", a, b)
	}
	return fmt.Sprintf("%s and %s spent time together.", a, b)
}
