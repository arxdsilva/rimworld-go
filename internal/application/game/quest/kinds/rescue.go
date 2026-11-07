package kinds

import (
	"rimworld-go/internal/application/game/quest"
	qd "rimworld-go/internal/domain/quest"
)

// Rescue is the prisoner to rescue from a camp (QST-065..069).
// A stub until QST-2b: it is never offered.
type Rescue struct{}

func (Rescue) Def() qd.Def                                             { return qd.DefOf(qd.Rescue) }
func (Rescue) CanOffer(quest.Ctx) bool                                 { return false }
func (Rescue) Build(quest.Ctx, *qd.Quest) bool                         { return false }
func (Rescue) Accepted(quest.Ctx, *qd.Quest)                           {}
func (Rescue) Signal(quest.Ctx, *qd.Quest, quest.Signal) quest.Verdict { return quest.VerdictNone }
func (Rescue) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict         { return quest.VerdictNone }
func (Rescue) Ended(quest.Ctx, *qd.Quest)                              {}
func (Rescue) Detail(qd.Quest) []quest.Line                            { return nil }
