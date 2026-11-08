package care

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Do runs one tick of a care job the game's job switch hands over
// (SIM-063…065): feeding a patient, stripping, visiting the sick,
// carrying a downed prisoner back to bed. Each driver lives in its own
// file (feed.go, strip.go, visit.go, prisonbed.go).
func (c *Care) Do(id entity.EntityID, j job.Record) {
	switch j.Kind {
	case job.FeedPatient:
		c.Feed.Do(id, j)
	case job.Strip:
		if j.Count == helpMark {
			c.Help.Do(id, j) // help.go (QST-055)
			return
		}
		if j.Count == freeMark {
			c.Free.Do(id, j) // free.go (QST-067)
			return
		}
		c.Strip.Do(id, j)
	case job.VisitSick:
		c.Visits.Do(id, j)
	case job.CarryToPrisonBed:
		c.PrisonBeds.Do(id, j)
	}
}
