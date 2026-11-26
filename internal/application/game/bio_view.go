// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/dossierview"
	"rimworld-go/internal/application/game/tradeview"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Bio is a colonist's identity for the inspector's Bio tab (NG-1 §2.12,
// §5.4): roster.BioOf over its bio with the live skills (levels, XP progress,
// passions). Pawns without a bio report false. Weekly learning is
// Dossier's LearnedWeek. Owned by PAWN-G.
func (g *Game) Bio(id entity.EntityID) (BioView, bool) { return g.bioView(id) }

// Dossier is the colonist card's sim-side facts for one id: animals too,
// and no scan (RHUD). It reads the state only.
func (g *Game) Dossier(id entity.EntityID) (dossierview.Card, bool) {
	c, now := dossierview.New(), uint64(g.clock.Now())
	m, _ := g.motion.Get(id)
	c.StunTicks, c.StaggerTicks = dossierview.TicksLeft(g.sk.stun[id], now), dossierview.TicksLeft(m.StaggerUntil, now)
	c.Stats = stat.Base()
	if fx := g.sk.stats[id]; fx != nil {
		c.Stats = fx.sheet
	}
	if a, ok := g.animals.Get(id); ok {
		fillAnimalCard(g, &c, id, a, now)
		return c, true
	}
	if !g.isPawn(id) {
		return dossierview.Card{}, false
	}
	for _, s := range [...]stat.ID{stat.ShootAcc, stat.MoveSpeed, stat.WorkSpeed} {
		c.Stats[s] = g.statOf(id, s)
	}
	fillPawnCard(g, &c, id, now)
	return c, true
}

// colonyEmblem is the player's emblem: the colony has no row among the
// factions.
func colonyEmblem() string { return factionView(faction.Faction{Type: faction.Player}).Emblem }

func fillAnimalCard(g *Game, c *dossierview.Card, id entity.EntityID, a animal.State, now uint64) {
	c.AgeDayOfYear = int(g.clock.Calendar().CalendarDays(uint64(animal.AgeDays(a.Born, now, a.Species, g.clock.Calendar()) % clock.DaysPerYear)))
	c.AnimalFaction(a.Species, a.Tame, g.hostile(a) || a.Enraged(now))
	if g.worldSt != nil {
		c.PackFromVisit(g.worldSt.visitGroups, g.ng.factions, id)
	}
	if a.Mode == animal.Attack && a.Target != 0 {
		c.AttackTarget = g.creatureName(a.Target)
	}
	c.EatSpeedPct = 100
}

func fillPawnCard(g *Game, c *dossierview.Card, id entity.EntityID, now uint64) {
	if p, ok := g.personBio(id); ok {
		c.AgeDayOfYear = int(g.clock.Calendar().CalendarDays(uint64(max(0, int(p.AgeDays)%pawngen.DaysPerYear))))
	}
	c.Relation = pawnRelation(g, id)
	if c.Relation == dossierview.RelationOwn {
		c.FactionEmblem = colonyEmblem()
	} else if p := g.personOf(id); p != nil {
		if f := g.factionOf(p.Faction); f != nil {
			v := factionView(*f)
			c.Faction, c.FactionEmblem = v.Name, v.Emblem
		} else {
			c.HiddenOrigin(p.Origin)
		}
	}
	if t := g.trade.caravan; t != nil && t.Trader == id {
		c.Role = tradeview.LowerLabel(trade.DefOf(t.Kind).Label)
	}
	c.BreakAt = dossierview.BreakLevels(g.mind.Thresholds(id))
	if b, ok := g.breaks[id]; ok {
		c.BreakTier = mentalbreak.TierOf(b.kind)
	}
	if sk, ok := activeSkill(g, id); ok {
		c.ActiveSkill = int(sk)
	}
	if lg := g.sk.learned[id]; lg != nil {
		day := int32(now / clock.TicksPerDay)
		c.Saturated, c.LearnedWeek = lg.Saturated(day, skill.SoftCapDay), lg.LastWeek(day)
	}
	c.EatSpeedPct = int(body.EatingSpeed(g.bodies[id])) / 10
	if j, _ := g.jobs.Get(id); j.Active() {
		c.WorkType = workTypeOfJob(g, j)
		if j.Target != 0 && (j.Kind == job.Attack || j.Kind == job.Hunt || j.Kind == job.Brawl) {
			c.AttackTarget = g.creatureName(j.Target)
		}
	}
	c.ExtraLines = dossierview.Lines(dossierview.Facts{
		Animals: int(g.skillOf(id, skill.Animals).Level), Cooking: int(g.skillOf(id, skill.Cooking).Level),
		Medicine: int(g.skillOf(id, skill.Medicine).Level), Melee: g.meleeOf(id),
		MineYieldPct:         g.miningYieldPermil(id) * g.diffScale(DiffMine, 100) / 1000,
		CropYieldPct:         g.plantYield(id) * g.diffScale(DiffCrop, 100) / 1000,
		ButcherDifficultyPct: g.diffScale(DiffButcher, 100),
	})
}

// pawnRelation is how person id stands to the colony.
func pawnRelation(g *Game, id entity.EntityID) dossierview.Relation {
	_, foreign := g.raiders[id]
	switch {
	case g.prison.Is(id):
		return dossierview.RelationHeld
	case g.isAlly(id):
		return dossierview.RelationAlly
	case g.visitor(id) || foreign && !g.isRaider(id):
		return dossierview.RelationNone
	case foreign:
		return dossierview.RelationFoe
	}
	return dossierview.RelationOwn
}

// workTypeOfJob is the work type of job j: a bill's is its recipe's, a wild
// plant's harvest is Plant cut (HRV-003).
func workTypeOfJob(g *Game, j job.Record) int {
	if p, ok := g.plants.Get(j.Target); ok && j.Kind == job.Harvest && wildPlant(g, p) {
		return dossierview.WorkTypeOf(job.Cut)
	}
	if !dossierview.RunsBill(j.Kind) {
		return dossierview.WorkTypeOf(j.Kind)
	}
	if bills := g.craft.Bills.Of(j.Dest); j.BillIndex >= 0 && j.BillIndex < len(bills) {
		return int(g.craft.Bills.Book().DefOf(bills[j.BillIndex].Recipe).WorkType())
	}
	return dossierview.NoneID
}

// applyRenamePawn is command.RenamePawn, the vanilla rename dialog: a
// colonist's or guest's bio takes the first, nick and last names (each by the
// wizard's rule; the first one is required) and its display name follows,
// the nickname when there is one. Anyone else, or a refused name, changes
// nothing.
func applyRenamePawn(g *Game, c command.Command) {
	id := entity.EntityID(c.Pawn)
	if _, foreign := g.raiders[id]; foreign || !g.isPawn(id) {
		return
	}
	p, ok := g.personBio(id)
	if !ok {
		return
	}
	p, ok = dossierview.RenamePerson(p, c.Name)
	if !ok {
		return
	}
	if _, own := g.sk.bios[id]; own {
		g.sk.bios[id] = p
	} else if q := g.personOf(id); q != nil {
		q.Bio = p
	}
	g.names.Set(id, p.DisplayName())
	g.viewGen++
}

// applySetName is command.SetName (RHUD-025): a colonist or tame animal takes
// the name, and "" gives back its generated one. A colonist's is its bio's,
// which a rename leaves alone; an animal's was rolled, so its first rename
// keeps it aside (skillState.petNames, saved). Anyone else, or a name the
// wizard would refuse, changes nothing.
func applySetName(g *Game, c command.Command) {
	id := entity.EntityID(c.Pawn)
	name, ok := dossierview.CleanName(c.Name)
	if !ok {
		return
	}
	if a, tameable := g.animals.Get(id); tameable {
		if !a.Tame {
			return
		}
		if name = g.sk.renamePet(id, a.Name, name); name == "" {
			return
		}
		a.Name = name
		g.animals.Set(id, a)
		g.viewGen++
		return
	}
	if _, foreign := g.raiders[id]; foreign || !g.isPawn(id) {
		return
	}
	if name == "" {
		p, ok := g.personBio(id)
		if !ok || p.DisplayName() == "" {
			return
		}
		name = p.DisplayName()
	}
	g.names.Set(id, name)
	g.viewGen++
}
