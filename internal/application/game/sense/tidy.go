package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// TidyRoles is the game's side of Tidy; CSE-017…027 fill it in.
type TidyRoles interface{}

// Tidy cleans a room before work in it (CSE-017…027).
type Tidy struct {
	s *Sense
	r TidyRoles
}

func newTidy(s *Sense, r TidyRoles) Tidy { return Tidy{s: s, r: r} }

// Before may swap the job about to start for a tidy first; ok false keeps it.
func (t Tidy) Before(id entity.EntityID, rec job.Record) (job.Record, bool) { return rec, false }

// After runs when a job has ended and been cleared; true: it started the job
// a tidy interrupted, and the pawn has a job again.
func (t Tidy) After(id entity.EntityID, j job.Record) bool { return false }

// Forget drops the tidy state of an interrupted pawn.
func (t Tidy) Forget(id entity.EntityID) {}

// SetBench is CSE-019: enabled false opts the bench out of tidying.
func (t Tidy) SetBench(bench entity.EntityID, enabled bool) {}

// Do runs one tick of job.CleanRoom.
func (t Tidy) Do(id entity.EntityID, j job.Record) {}
