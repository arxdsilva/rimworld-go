package kinds

import (
	"rimworld-go/internal/application/game/quest"
	qd "rimworld-go/internal/domain/quest"
)

// Camp is the hostile camp to clear (QST-060..064).
// A stub until QST-2b: it is never offered.
type Camp struct{}

func (Camp) Def() qd.Def                                             { return qd.DefOf(qd.Camp) }
func (Camp) CanOffer(quest.Ctx) bool                                 { return false }
func (Camp) Build(quest.Ctx, *qd.Quest) bool                         { return false }
func (Camp) Accepted(quest.Ctx, *qd.Quest)                           {}
func (Camp) Signal(quest.Ctx, *qd.Quest, quest.Signal) quest.Verdict { return quest.VerdictNone }
func (Camp) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict         { return quest.VerdictNone }
func (Camp) Ended(quest.Ctx, *qd.Quest)                              {}
func (Camp) Detail(qd.Quest) []quest.Line                            { return nil }
