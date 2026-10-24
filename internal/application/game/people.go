package game

import "rimworld-go/internal/domain/sim/entity"

// peopleRegistry is every person the colony knows of (ENG-003, M3):
// colonists, raiders, visitors, prisoners and off-map relatives, with
// their bios and where they are. nil until the first person is
// registered; saved as People.
type peopleRegistry struct{}

// peopleSave is peopleRegistry as saved (filled by M3).
type peopleSave struct {
	Version uint8 // format of this block; 0 = empty
}

// raiderPerson registers raider id of faction fac (0: a legacy raid) as a
// world person, right after it spawns.
func (g *Game) raiderPerson(id entity.EntityID, fac int) {
	// TODO(M3)
}

// seedFamilies gives the founding colonists their starting relations,
// once, when the colony is founded.
func (g *Game) seedFamilies() {
	// TODO(M3)
}

// exportPeople is the People save field (nil: nothing yet).
func (g *Game) exportPeople() *peopleSave {
	if g.people == nil {
		return nil
	}
	return nil // TODO(M3)
}

// importPeople restores exportPeople's state (nil: fresh).
func (g *Game) importPeople(s *peopleSave) {
	if s == nil {
		return
	}
	// TODO(M3)
}
