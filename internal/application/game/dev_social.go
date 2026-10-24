package game

import (
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/sim/entity"
)

// Dev hooks for the social systems (scenarios and captures only; the
// golden scenarios never call them). Each reports false on an unknown
// name or id and changes nothing then.

var devPairKinds = map[string]social.PairKind{
	"lover": social.PairLover, "fiance": social.PairFiance, "spouse": social.PairSpouse,
	"exlover": social.PairExLover, "exspouse": social.PairExSpouse,
}

var devMemKinds = map[string]social.MemKind{
	"chitchat": social.MemChitchat, "deep_talk": social.MemDeepTalk,
	"slighted": social.MemSlighted, "insulted": social.MemInsulted,
	"kind_words": social.MemKindWords, "rebuffed": social.MemRebuffedMe,
	"cheated": social.MemCheatedOnMe, "broke_up": social.MemBrokeUpWithMe,
	"divorced": social.MemDivorcedMe, "angering_fight": social.MemAngeringFight,
	"harmed": social.MemHarmedMe, "rescued": social.MemRescuedMe,
}

// DevSetPair sets the a-b couple state: "lover", "fiance", "spouse",
// "exlover" or "exspouse". It skips the transition table.
func (g *Game) DevSetPair(a, b entity.EntityID, kind string) bool {
	k, ok := devPairKinds[kind]
	if !ok || a == b || !g.isPawn(a) || !g.isPawn(b) {
		return false
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.ensurePerson(a)
	g.ensurePerson(b)
	now := uint64(g.clock.Now())
	g.setPair(a, b, social.Pair{Kind: k, Since: now, LastChange: now})
	g.refreshSig(a)
	g.refreshSig(b)
	return true
}

// DevAddOpinionMemory makes observer remember kind about subject
// ("insulted", "deep_talk", "slighted", "kind_words", ...).
func (g *Game) DevAddOpinionMemory(observer, subject entity.EntityID, kind string) bool {
	k, ok := devMemKinds[kind]
	if !ok || observer == subject || !g.isPawn(observer) || !g.isPawn(subject) {
		return false
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.ensurePerson(observer)
	g.ensurePerson(subject)
	g.addOpinionMemory(observer, subject, k, uint64(g.clock.Now()))
	return true
}

// DevBond bonds tame animal with colonist pawn (and makes it the master
// when it has none).
func (g *Game) DevBond(animal, pawn entity.EntityID) bool {
	a, ok := g.animals.Get(animal)
	if !ok || !a.Tame || !g.bondable(pawn) {
		return false
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	a.Bond = uint64(pawn)
	if a.Master == 0 {
		a.Master = uint64(pawn)
	}
	g.animals.Set(animal, a)
	return true
}

// DevScar gives pawn a permanent wound (a scar) on the named body part
// ("jaw", "left eye", ...; a face part makes it disfigured).
func (g *Game) DevScar(pawn entity.EntityID, part string) bool {
	p, ok := partByLabel(part)
	if !ok || !g.isPawn(pawn) {
		return false
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if g.bodies == nil {
		g.bodies = make(map[entity.EntityID]*body.Body)
	}
	b := g.bodies[pawn]
	if b == nil {
		b = &body.Body{}
		g.bodies[pawn] = b
	}
	b.Wounds = append(b.Wounds, body.Wound{Part: p, Dmg: 1, Kind: body.Cut, Tended: true, At: uint64(g.clock.Now()), Permanent: true})
	g.ensurePerson(pawn)
	g.refreshSig(pawn)
	return true
}

// DevLosePart destroys pawn's named part ("left leg", "left eye", ...) as
// a hit would: it and the parts below it go missing with their wounds,
// then Injury, the capacities and the stat sheet follow (SIM-049). A part
// already gone, a vital part or a dead pawn reports false.
func (g *Game) DevLosePart(pawn entity.EntityID, part string) bool {
	p, ok := partByLabel(part)
	if !ok || !g.isPawn(pawn) {
		return false
	}
	v, ok := g.vital.Get(pawn)
	if !ok || v.Status == health.Dead {
		return false
	}
	op := body.Operation{Kind: body.Amputate, Part: p}
	if !g.viewBody(pawn).CanOperate(op) {
		return false
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	b := g.ensureBody(pawn, &v)
	b.Apply(op, body.Success, func(int) int { return 0 }, uint64(g.clock.Now())) // a success rolls nothing
	g.bodyChanged(pawn, b, &v)                                                   // body.go
	if v.Status == health.Alive && unconscious(b) {
		g.down(pawn, health.CauseInjury)
	}
	g.ensurePerson(pawn)
	g.refreshSig(pawn) // a lost face part disfigures
	return true
}

// DevSpawnImplants drops one of each implant kind on free tiles near
// pawn near, nearest first (SIM-049).
func (g *Game) DevSpawnImplants(near entity.EntityID) bool {
	pos, ok := g.positions.Get(near)
	if !ok || !g.isPawn(near) {
		return false
	}
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	used := make([][2]int, 0, len(implantKinds))
	for _, k := range implantKinds {
		x, y, ok := g.freeLandNear(pos.X, pos.Y, 8, used) // arrival.go
		if !ok {
			x, y = pos.X, pos.Y // crowded: they pile on the pawn's tile
		}
		used = append(used, [2]int{x, y})
		g.dropStack(k, 1, x, y) // world.go
	}
	return true
}

func partByLabel(name string) (body.Part, bool) {
	for p := body.Part(0); p < body.NumParts; p++ {
		if p.Label() == name {
			return p, true
		}
	}
	return 0, false
}
