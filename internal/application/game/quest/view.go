package quest

import (
	"slices"

	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/economy/trade"
)

// View is one quest in the read model (QST-120..130): copies only.
type View struct {
	ID     qd.ID
	Kind   qd.KindID
	State  qd.State
	Reason qd.Reason
	Stars  uint8
	// Giver and Target are 1 + faction index (0: none).
	Giver, Target                  int
	OfferedAt, AcceptedAt, EndedAt uint64
	AcceptBy, Deadline             uint64
	Dismissed                      bool
	Rewards                        []RewardView
	// Chosen is the accepted option (-1: none yet).
	Chosen int
	// Detail is the kind's lines (lang keys and args).
	Detail []Line
	// Tile is the site tile (-1: none).
	Tile int
}

// RewardView is one "accept for" option as display data (QST-128).
type RewardView struct {
	Lines    []trade.Line
	Goodwill int
	// Pawn previews a pawn reward (HasPawn) from its seed (QST-087).
	HasPawn  bool
	Pawn     PawnPreview
	SiteLoot bool
	Value    int
}

// View is every quest, ids ascending (nil for none, as after a load of
// an idle book). The game memoizes it by Version.
func (q *Quests) View() []View {
	if len(q.list) == 0 {
		return nil
	}
	out := make([]View, 0, len(q.list))
	for _, n := range q.list {
		v := View{ID: n.ID, Kind: n.Kind, State: n.State, Reason: n.Reason, Stars: n.Stars,
			Giver: n.Giver, Target: n.Target, OfferedAt: n.OfferedAt, AcceptedAt: n.AcceptedAt,
			EndedAt: n.EndedAt, AcceptBy: n.AcceptBy, Deadline: n.Deadline, Dismissed: n.Dismissed,
			Chosen: int(n.Chosen), Tile: siteTile(n)}
		for _, r := range n.Rewards {
			rv := RewardView{Lines: slices.Clone(r.Lines), Goodwill: r.Goodwill, SiteLoot: r.SiteLoot, Value: r.Value}
			if r.Pawn != 0 {
				rv.HasPawn, rv.Pawn = true, q.r.Pawns.Preview(r.Pawn)
			}
			v.Rewards = append(v.Rewards, rv)
		}
		if k := q.kindOf(n.Kind); k != nil {
			v.Detail = k.Detail(n)
		}
		out = append(out, v)
	}
	return out
}

// AcceptsGoodwill: goodwill from faction ref (1 + index) is accepted as a
// reward (the preference dialog, QST-083).
func (q *Quests) AcceptsGoodwill(ref int) bool { return !q.declines(ref - 1) }
