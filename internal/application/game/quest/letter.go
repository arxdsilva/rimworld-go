package quest

import qd "rimworld-go/internal/domain/quest"

// LetterKind is which quest letter it is (QST-131..134).
type LetterKind uint8

const (
	// LetterOffer: a new quest (offered, or active at once for an
	// AutoAccept kind).
	LetterOffer LetterKind = iota + 1
	// LetterFailed names the reason (Q38, Q39).
	LetterFailed
	// LetterSucceeded: the quest succeeded (its reward is on its way).
	LetterSucceeded
)

// ActionKind is a letter's action row.
type ActionKind uint8

const (
	// ActAccept accepts the offer with reward option Choice (Accept).
	ActAccept ActionKind = iota + 1
	// ActView opens the Quests window on the quest (QST-131).
	ActView
	// ActClose dismisses the letter.
	ActClose
)

// Action is one letter action, as data: the glue turns it into a command.
type Action struct {
	Kind   ActionKind
	Quest  qd.ID
	Choice int
}

// Letter is a quest letter, as data: the glue picks its lang keys from
// QuestKind, Kind and Reason. An expiry posts none (Q36).
type Letter struct {
	Kind      LetterKind
	Quest     qd.ID
	QuestKind qd.KindID
	State     qd.State
	Reason    qd.Reason
	// Tile is the quest's site tile (-1: none).
	Tile    int
	Actions []Action
}

// post sends quest n's letter of kind lk. An offer awaiting accept gets
// one Accept row per reward option (one with choice -1 when it has none),
// then View and Close; every other letter View and Close.
func post(l Letters, lk LetterKind, n qd.Quest) {
	var acts []Action
	if lk == LetterOffer && n.State == qd.Offered {
		if len(n.Rewards) == 0 {
			acts = append(acts, Action{Kind: ActAccept, Quest: n.ID, Choice: -1})
		}
		for c := range n.Rewards {
			acts = append(acts, Action{Kind: ActAccept, Quest: n.ID, Choice: c})
		}
	}
	acts = append(acts, Action{Kind: ActView, Quest: n.ID}, Action{Kind: ActClose, Quest: n.ID})
	l.Post(Letter{Kind: lk, Quest: n.ID, QuestKind: n.Kind, State: n.State, Reason: n.Reason,
		Tile: siteTile(n), Actions: acts})
}

// siteTile is quest n's site tile: its first site, talk or delivery
// part's (-1: none).
func siteTile(n qd.Quest) int {
	for _, p := range n.Parts {
		switch p.Kind {
		case qd.PartSite, qd.PartTalk, qd.PartDelivery:
			if p.Tile >= 0 {
				return p.Tile
			}
		}
	}
	return -1
}
