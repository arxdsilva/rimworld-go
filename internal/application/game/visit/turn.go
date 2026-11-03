package visit

import (
	"fmt"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// Visitors whose faction turns on the colony (MAP-18; the wiki has no page
// for it, this is how the game plays). They stop visiting and walk off the
// map as enemies, defending themselves. The record stays until the person is
// gone, so the trader's goods and the group's room on the map end as they
// always did. Turned is sticky for the visit: a faction that calms down
// meanwhile changes nothing, so nobody flaps between guest and enemy.

// TurnedTitle is the letter that tells of it, one per group.
const TurnedTitle = "Visitors turned hostile"

// Turning is the simulation's side of a visitor turned enemy.
type Turning interface {
	// Enemy reports turned visitor id already walks out as a hostile: on the
	// raider list, not dead, nobody's prisoner.
	Enemy(id entity.EntityID) bool
	// TurnHostile makes visitor id a hostile on its way out.
	TurnHostile(id entity.EntityID, now uint64)
	// ShootBack runs enemy id's self defence, and ends a fight it would have
	// to chase.
	ShootBack(id entity.EntityID, now uint64)
	// Rescuing reports someone is carrying or coming for downed visitor id:
	// the rescue ends in a bed first.
	Rescuing(id entity.EntityID) bool
	// TraderTurned closes the trade with faction ref's caravan; false when
	// it has none on the map.
	TraderTurned(ref int) bool
}

// notice turns every group whose faction is hostile now. The check is one
// comparison per unturned record.
func (v Visits) notice(list []Rec) {
	for i := range list {
		if list[i].Turned {
			continue
		}
		if f := ofRef(v.h.Factions(), list[i].Faction); f != nil && f.Relation == faction.Hostile {
			v.turnGroup(list, list[i].Faction, list[i].ID, f.Name)
		}
	}
}

// turnGroup marks faction ref's visitors turned and says so, once.
func (v Visits) turnGroup(list []Rec, ref int, first entity.EntityID, name string) {
	for i := range list {
		if list[i].Faction == ref {
			list[i].Turned = true
		}
	}
	text := fmt.Sprintf("%s has turned on you. Its people here have stopped visiting and are making for the edge of the map, ready to defend themselves. Your colonists and turrets may treat them as enemies.", name)
	if v.h.TraderTurned(ref) {
		text += " Its trader will not deal with you now."
	}
	v.h.Letter("caution", TurnedTitle, text, first)
}

// walkOut runs an enemy for one tick: the raid AI walks it out and strikes
// back at whoever stands beside it (raid/tactics.go), this adds its answer
// to being shot; one that can't get out slips away as any visitor does.
func (v Visits) walkOut(r *Rec, now uint64) bool {
	if now >= r.LeaveAt+giveUp && v.h.Standing(r.ID) {
		v.h.Depart(r.ID)
		return false
	}
	v.h.ShootBack(r.ID, now)
	return true
}

// turn runs a turned visitor still a guest: a guest in a bed or a rescuer's
// arms stays one until it is well; anyone else, down or up, becomes an enemy
// now.
func (v Visits) turn(r *Rec, now uint64) bool {
	h := v.h
	h.Hold(r.ID, now)
	if h.GuestStep(r.ID) || h.Rescuing(r.ID) {
		return true
	}
	h.PutDown(r.ID) // carry.go: a fight is coming, the friend it held is on its own
	r.Fetch = 0
	h.TurnHostile(r.ID, now)
	r.Leaving, r.LeaveAt = true, now
	return true
}
