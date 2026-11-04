package care

import (
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

// roomsByX is an Urgency: a tile's room is x / 10.
type roomsByX struct {
	hours map[entity.EntityID]int
}

func (r roomsByX) RoomAt(x, _ int) int { return x / 10 }
func (r roomsByX) BleedDeathHours(id entity.EntityID) int {
	if h, ok := r.hours[id]; ok {
		return h
	}
	return -1
}

func TestNoCleanWithNextPatientHere(t *testing.T) {
	t.Parallel()
	c, h := scene()
	h.pawn(3, 6, 6)
	h.downed(3, 50)
	if !c.Doctor.Urgent(doc, patient, roomsByX{}) {
		t.Fatal("a patient lying in the same room comes first")
	}
	h.pos[3] = [2]int{16, 6}
	if c.Doctor.Urgent(doc, patient, roomsByX{}) {
		t.Fatal("not bleeding, in another room: time to tidy")
	}
}

func TestNoCleanWhenPatientBleeding(t *testing.T) {
	t.Parallel()
	c, h := scene()
	h.pawn(3, 26, 6)
	h.downed(3, 50)
	h.bleed[3] = 100
	if !c.Doctor.Urgent(doc, patient, roomsByX{map[entity.EntityID]int{3: 5}}) {
		t.Fatal("bleeding out within 6 hours, anywhere, comes first")
	}
	if c.Doctor.Urgent(doc, patient, roomsByX{map[entity.EntityID]int{3: 6}}) {
		t.Fatal("6 hours is time enough")
	}
	h.owners[3] = 77
	if c.Doctor.Urgent(doc, patient, roomsByX{map[entity.EntityID]int{3: 5}}) {
		t.Fatal("another doctor has that one")
	}
}
