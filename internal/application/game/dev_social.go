package game

import (
	"rimworld-go/internal/domain/pawn/body"
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

func partByLabel(name string) (body.Part, bool) {
	for p := body.Part(0); p < body.NumParts; p++ {
		if p.Label() == name {
			return p, true
		}
	}
	return 0, false
}
