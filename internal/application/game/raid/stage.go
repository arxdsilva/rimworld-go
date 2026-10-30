package raid

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/combat/weapon"
	raiddef "rimworld-go/internal/domain/raid"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Staged raids (DEF-4, StageThenAssault). wiki: Raid, the raiders stand
// around in a small group near where they came in, and attack once they
// see a colonist nearby or their assault timer (10 to 15 hours, CBT-4)
// runs out. Here the group gathers at a spot raiddef.StageSpotIn tiles in
// from its edge; it storms when a colonist comes within SightRange of a
// raider or when the timer runs out. A raider walks back to the spot only
// past raiddef.StageWander tiles, so it never shuffles to and fro; the
// storm is one way.

// stageEvery staggers the staging checks (a colonist in sight, gathered).
const stageEvery = 30

// stage runs a staged raider at (x, y) before its group storms. It
// reports whether it acted.
func (rs *Raiders) stage(id entity.EntityID, r Raider, j job.Record, x, y int, now uint64) bool {
	h := rs.h
	grp := &h.Groups()[r.Group]
	if grp.Plan != Stage || grp.Stormed {
		return false
	}
	if grp.Started == 0 {
		grp.Started = now
		grp.Until = now + uint64(raiddef.StageLimitMin+h.Roll(raiddef.StageLimitRolls))*clock.TicksPerHour
		grp.CampX, grp.CampY = rs.stageSpot(x, y)
		grp.Camped = true
	}
	if (now+uint64(id))%stageEvery == 0 {
		_, cx, cy, seen := h.NearestColonist(x, y)
		if seen && weapon.Distance(x, y, cx, cy) <= SightRange {
			rs.storm(r.Group, x, y, now, "The gathered raiders are attacking!")
			return false
		}
	}
	if now >= grp.Until {
		rs.storm(r.Group, x, y, now, "The gathered raiders are attacking!")
		return false
	}
	if weapon.Distance(x, y, grp.CampX, grp.CampY) > raiddef.StageWander && !j.Active() {
		if tx, ty, ok := h.WalkableNear(grp.CampX+h.Roll(3)-1, grp.CampY+h.Roll(3)-1, 2); ok {
			h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty})
		}
	}
	return true
}

// stageSpot is the staging spot for a raid that came in at (x, y):
// StageSpotIn tiles from there towards the colony.
func (rs *Raiders) stageSpot(x, y int) (int, int) {
	hx, hy := rs.h.Home()
	d := max(1, weapon.Distance(x, y, hx, hy))
	in := min(raiddef.StageSpotIn, d)
	sx, sy := x+(hx-x)*in/d, y+(hy-y)*in/d
	if cx, cy, ok := rs.h.WalkableNear(sx, sy, 4); ok {
		return cx, cy
	}
	return x, y
}

// storm turns a staged or besieging group onto the colony for good: Until
// becomes the storm's tick (raiders give up a day after it).
func (rs *Raiders) storm(gi, x, y int, now uint64, text string) {
	grp := &rs.h.Groups()[gi]
	grp.Stormed, grp.Until = true, now
	title := "Raid"
	if grp.Plan == Siege {
		title = "Siege assault"
		for _, id := range rs.h.RaiderIDs() { // the mortar crews leave their guns
			if j, _ := rs.h.Job(id); j.Kind == job.ManMortar && rs.h.Raider(id).Group == gi {
				rs.h.Interrupt(id, job.Cancelled)
			}
		}
	}
	rs.h.Threat(title, x, y, text)
	rs.log(text)
}

// log adds text to the combat log (nil in unit tests without one).
func (rs *Raiders) log(text string) {
	if rs.ch != nil {
		rs.ch.Log(text)
	}
}
