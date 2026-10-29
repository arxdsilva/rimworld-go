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
// M5 adds the partner thoughts: feelings for the main partner (opinion/10,
// clamped to +-10) and sleeping alone while that partner is on the map.
func (g *Game) socialThoughts(id entity.EntityID) []mood.Thought {
	out := g.prisonerThoughts(id)            // prisoners.go
	out = append(out, g.bondThoughts(id)...) // bonds.go
	if g.soc == nil {
		return out
	}
	p := g.partnerOf(id) // social_romance.go
	if p == 0 {
		return out
	}
	if v := partnerMood(g.opinion(id, p)); v != 0 {
		out = append(out, mood.Thought{Kind: mood.PartnerOpinion, Label: mood.DefOf(mood.PartnerOpinion).Label, Value: v})
	}
	if g.sleptAlone(id, p) {
		d := mood.DefOf(mood.SleptAlone)
		out = append(out, mood.Thought{Kind: mood.SleptAlone, Label: d.Label, Value: d.Value})
	}
	return out
}

// partnerMood is the partner-opinion thought's value: the opinion (centi)
// over ten, whole points, clamped to +-partnerMoodCap.
func partnerMood(opinion int32) int {
	return max(-partnerMoodCap, min(partnerMoodCap, int(opinion/1000)))
}

const partnerMoodCap = 10

// sleptAlone reports id's last sleep was not in the bed its partner p
// slept in, while p is on the map (beds.go keeps sleptAt per sleep).
func (g *Game) sleptAlone(id, p entity.EntityID) bool {
	if !g.onMap(p) || !g.onMap(id) {
		return false
	}
	mine, ok := g.sleptAt[id]
	if !ok {
		return false // not slept yet
	}
	theirs, ok := g.sleptAt[p]
	return !ok || mine == 0 || mine != theirs
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
	target := int32(g.thoughtLevel(id)) * 1000
	s := g.socState()
	cur, ok := s.moodM[id]
	if !ok {
		s.moodM[id] = target
		return
	}
	s.moodM[id] = mood.Approach(cur, target, moodPeriod)
}
