package kinds

import (
	"rimworld-go/internal/application/game/quest"
	qd "rimworld-go/internal/domain/quest"
)

// Refugee is the downed refugee at a site (QST-055..059).
// A stub until QST-2b: it is never offered.
type Refugee struct{}

func (Refugee) Def() qd.Def                                             { return qd.DefOf(qd.Refugee) }
func (Refugee) CanOffer(quest.Ctx) bool                                 { return false }
func (Refugee) Build(quest.Ctx, *qd.Quest) bool                         { return false }
func (Refugee) Accepted(quest.Ctx, *qd.Quest)                           {}
func (Refugee) Signal(quest.Ctx, *qd.Quest, quest.Signal) quest.Verdict { return quest.VerdictNone }
func (Refugee) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict         { return quest.VerdictNone }
func (Refugee) Ended(quest.Ctx, *qd.Quest)                              {}
func (Refugee) Detail(qd.Quest) []quest.Line                            { return nil }
