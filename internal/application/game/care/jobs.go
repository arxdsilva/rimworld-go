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
		c.Strip.Do(id, j)
	case job.VisitSick:
		c.Visits.Do(id, j)
	case job.CarryToPrisonBed:
		c.PrisonBeds.Do(id, j)
	}
}

// endJob completes j for id (the W3 stubs use it until their agents land).
func (c *Care) endJob(id entity.EntityID, j job.Record) {
	j.State = job.Complete
	c.h.SetJob(id, j)
}
