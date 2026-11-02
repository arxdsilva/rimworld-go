package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// The rocket battery (AX-5; wiki: Rocketswarm launcher) is manned like a
// mortar and shares its path (Mortars: the crew call, ManMortar, the
// Set target / Clear target orders and their pane). It never picks a
// target itself: the player sets one, a drafted colonist on its crew spot
// (or the one the order named, or the nearest idle drafted one) mans it,
// and while it is powered, loaded and recharged it aims for the gun's 60
// ticks, then fires RocketVolley rockets 10 ticks apart at the target,
// each landing within the forced-miss radius (5.75 tiles) of it on its own
// roll. The volley clears the target (the player aims every volley), empties
// it (Mortar.Fired counts the rockets since the last reload) and starts
// its weapon.RocketRecharge; a colonist then reloads the full 180
// chemfuel (rearm.go). The crew stands down as a mortar's does, so it
// never flaps.

// RocketReload is the chemfuel a reload takes: 180, /9 as every cost
// here. wiki: Rocketswarm launcher
const RocketReload = 20

// stepBattery is rocket battery id's tick.
func (m *Mortars) stepBattery(id entity.EntityID, b construction.Building, st *Mortar, now uint64) {
	h := m.h
	d := weapon.DefOf(weapon.SwarmRocket)
	if st.Crew != 0 && !m.crewing(st.Crew, id, false) {
		st.Crew, st.Aiming = 0, false
	}
	aim := st.Forced && weapon.ReachesAt(d, b.X, b.Y, st.TX, st.TY)
	if aim {
		st.SeenAt = now
	}
	if st.Crew == 0 {
		if aim && (now+uint64(id))%TurretScanEvery == 0 {
			sx, sy := spot(b)
			m.call(id, st, sx, sy, now)
		}
		return
	}
	cx, cy, _ := h.Pos(st.Crew)
	if j, _ := h.Job(st.Crew); j.State != job.Do || cx != j.TX || cy != j.TY {
		return // still walking up
	}
	if !aim {
		st.Aiming = false
		if now >= st.ManSince+MortarManMin && now >= st.SeenAt+MortarIdleRelease {
			m.standDown(st)
		}
		return
	}
	if !m.powered(id) {
		st.Aiming = false // a volley cut short by a brownout leaves it to be reloaded
		return
	}
	if !st.Aiming {
		if st.Fired > 0 || now < st.ReadyAt {
			return // empty, or recharging
		}
		st.Aiming, st.ReadyAt = true, now+uint64(d.Warmup)
	}
	if now < st.ReadyAt {
		return
	}
	lx, ly := m.scatter(st.TX, st.TY, d.ForcedMiss) // each rocket its own spread
	m.shots.Lob(st.Crew, weapon.SwarmRocket, b.X, b.Y, st.TX, st.TY, lx, ly)
	st.Fired++
	st.ReadyAt = now + uint64(d.BurstTicks)
	if st.Fired >= RocketVolley {
		st.Aiming, st.Forced, st.TX, st.TY = false, false, 0, 0
		st.ReadyAt = now + uint64(d.Cooldown)
	}
}

// powered: building id draws power now (no power device: never).
func (m *Mortars) powered(id entity.EntityID) bool {
	return m.pw != nil && m.pw.Wired() && m.pw.Powered(id)
}

// reloadWanted: rocket battery id has fired (some of) its volley and is
// not firing: it takes a full reload, once.
func (m *Mortars) reloadWanted(id entity.EntityID) bool {
	st := m.states[id]
	return st.Fired > 0 && !st.Aiming
}

// reload fills rocket battery id with a full volley.
func (m *Mortars) reload(id entity.EntityID) {
	st := m.states[id]
	st.Fired = 0
	m.set(id, st)
}
