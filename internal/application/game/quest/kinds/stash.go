package kinds

import (
	"rimworld-go/internal/application/game/quest"
	qd "rimworld-go/internal/domain/quest"
)

// Stash is the item stash site (QST-050..054).
// A stub until QST-2b: it is never offered.
type Stash struct{}

func (Stash) Def() qd.Def                                             { return qd.DefOf(qd.Stash) }
func (Stash) CanOffer(quest.Ctx) bool                                 { return false }
func (Stash) Build(quest.Ctx, *qd.Quest) bool                         { return false }
func (Stash) Accepted(quest.Ctx, *qd.Quest)                           {}
func (Stash) Signal(quest.Ctx, *qd.Quest, quest.Signal) quest.Verdict { return quest.VerdictNone }
func (Stash) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict         { return quest.VerdictNone }
func (Stash) Ended(quest.Ctx, *qd.Quest)                              {}
func (Stash) Detail(qd.Quest) []quest.Line                            { return nil }
