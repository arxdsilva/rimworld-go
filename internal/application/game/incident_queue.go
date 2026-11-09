package game

import (
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/pkg/msgfmt"
)

// The incident queue (EAI-030) and the chased refugee (EAI-031). The story
// queues an event for a later tick (QueueIncident, QueueRaid); phaseIncidents
// fires each entry once its tick comes, in the order queued, with no
// randomness of its own. The queue is saved with the incident state.

const (
	// refugeeSalt keys the refugee's person on the world seed and the tick
	// the plea was made.
	refugeeSalt = 0x5E7F1C
	// refugeeWait: the plea lapses (as a refusal) after this long.
	refugeeWait = clock.TicksPerDay
	// refugeeDelay: the pursuers arrive this long after the refugee is
	// taken in (original tuning).
	refugeeDelay = clock.TicksPerDay / 2
)

// QueueIncident queues event id to fire at the absolute tick at, with
// threat points (0: sized when it fires). The story calls it to schedule a
// rival faction's raid (QueueRaid).
func (g *Game) QueueIncident(id incident.ID, at uint64, points int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.incidents.Enqueue(incident.Queued{ID: id, At: at, Points: points})
}

// QueueRaid queues a raid of points threat points (0: the colony's current
// points) for the tick at: a rival faction's, or a refugee's pursuers.
func (g *Game) QueueRaid(at uint64, points int) { g.QueueIncident(incident.RivalRaid, at, points) }

// fireQueue fires every queued event due by now, in queue order.
func (g *Game) fireQueue(now uint64) {
	for {
		q, ok := g.incidents.PopDue(now)
		if !ok {
			return
		}
		g.fireQueued(q, now)
	}
}

// fireQueued makes queued event q happen. A queued event skips the
// cooldown and the day checks, as the storyteller already chose it.
func (g *Game) fireQueued(q incident.Queued, now uint64) {
	if q.ID == incident.RivalRaid {
		g.queuedRaid(q.Points, now)
		return
	}
	if g.fireIncident(q.ID, now) {
		g.incidents.Fired(q.ID, g.rel(now))
	}
}

// queuedRaid sends a raid of points threat points (0: today's) from a
// hostile faction.
func (g *Game) queuedRaid(points int, now uint64) {
	day := g.relDay(now)
	if !g.hasSetup() {
		if points <= 0 {
			points = g.raidPoints(day)
		}
		g.raidWith(g.raidKit(points), g.raid.Threats.Plan(day))
		return
	}
	if points <= 0 {
		points = g.threatPoints()
	}
	fi := faction.PickRaider(g.ng.factions, g.rng)
	tech := faction.Industrial
	if fi >= 0 {
		d := faction.DefOf(g.ng.factions[fi].Type)
		if d.PointsPct > 0 {
			points = points * d.PointsPct / 100
		}
		tech = d.Tech
	}
	g.raidFrom(g.raidKitOf(points, tech), g.raid.Threats.Plan(day), fi+1)
}

// queuePoints is the threat points a raid queued now carries.
func (g *Game) queuePoints(now uint64) int {
	if g.hasSetup() {
		return g.threatPoints()
	}
	return g.raidPoints(g.relDay(now))
}

// --- the chased refugee (EAI-031) ---------------------------------------------

// refugeeFits: a setup colony with a hostile faction to chase the refugee,
// someone to answer, and an edge for the refugee to come in at.
func (g *Game) refugeeFits() bool {
	if !primary(g) || !g.hasSetup() || !hostileRaider(g) || everyoneGone(g) {
		return false
	}
	_, _, ok := g.worldEdge()
	return ok
}

// refugeeSeed is the refugee's person: from the world and the plea's tick.
func (g *Game) refugeeSeed(tick uint64) uint64 { return (g.worldSeed() ^ refugeeSalt ^ tick) | 1 }

// refugeeChased posts the plea: a stranger asks for shelter from the
// enemies on their heels. It lapses after a day, as a refusal.
func (g *Game) refugeeChased(now uint64) bool {
	if !g.refugeeFits() {
		return false
	}
	hx, hy := g.home()
	g.post(MessageView{Tick: now, X: hx, Y: hy, Letter: true, Title: "Refugee asks for shelter", Kind: letters.Caution,
		Msg: msgfmt.Of("%s begs for shelter: armed riders are close behind. Take them in and the riders will follow; send them away and they go on alone.", quest.PreviewOf(g.refugeeSeed(now)).Name)})
	g.letters.EditLast(func(l *letters.Letter) { l.Until, l.Choices = now+refugeeWait, letters.ShelterChoices() })
	return true
}

// answerLetter acts on answer mode of letter id (command.AnswerLetter): the
// letter goes off the stack either way. An unknown letter, or an answer it
// does not offer, is ignored.
func (g *Game) answerLetter(id uint64, mode int) {
	l, ok := g.letters.Get(id)
	if !ok || mode < 0 || mode >= len(l.Choices) {
		return
	}
	g.letters.Remove(id)
	g.viewGen++
	if l.Choices[mode].Key == letters.Accept {
		g.shelterGiven(l)
	}
}

// shelterGiven takes the refugee of plea l in and queues their pursuers.
func (g *Game) shelterGiven(l letters.Letter) {
	ex, ey, ok := g.worldEdge()
	x, y, in := g.insetLandNear(ex, ey, nil)
	if !ok || !in {
		g.worldNote("There is no way for the refugee to come in.")
		return
	}
	id, ok := walkInColonist(g, quest.PawnOf(g.refugeeSeed(l.Tick)), x, y)
	if !ok {
		g.worldNote("There is no way for the refugee to come in.")
		return
	}
	now := uint64(g.clock.Now())
	letterf(g, letters.Good, "Refugee takes shelter", id, "%s has taken shelter in the colony. The riders chasing them will come.", g.personName(id))
	g.addHistory("joined", g.personName(id)+" took shelter in the colony.", id)
	g.QueueRaid(now+refugeeDelay, g.queuePoints(now))
}
