package game

import (
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
)

// Smoothed mood (ENG-003, M3): the mood a colonist shows and breaks on
// moves toward the thought total gradually (mood.Approach), stepped every
// moodPeriod ticks staggered by id and saved per pawn.
const (
	moodPeriod  = 250
	moodStagger = 13
)

// socialThoughts are the social and host-system thoughts of pawn id
// (partner, grief, drugs, bonds, body), appended in thoughts (ENG-003).
// Grief, insults and kind words are ordinary memories and show already;
// M5 adds the partner thoughts here.
func (g *Game) socialThoughts(id entity.EntityID) []mood.Thought {
	return nil
}

// smoothedMood is the mood moodOf reports: target, approached gradually;
// target itself until id's first step.
func (g *Game) smoothedMood(id entity.EntityID, target int) int {
	if g.soc == nil {
		return target
	}
	m, ok := g.soc.moodM[id]
	if !ok {
		return target
	}
	return int((m + 500) / 1000)
}

// stepMood moves id's smoothed mood one step toward its thought total (a
// missing value starts at the target; the dead drop theirs).
func (g *Game) stepMood(id entity.EntityID) {
	if v, ok := g.vital.Get(id); !ok || v.Status == health.Dead {
		if g.soc != nil {
			delete(g.soc.moodM, id)
		}
		return
	}
	target := int32(mood.Level(g.thoughts(id))) * 1000
	s := g.socState()
	cur, ok := s.moodM[id]
	if !ok {
		s.moodM[id] = target
		return
	}
	s.moodM[id] = mood.Approach(cur, target, moodPeriod)
}
