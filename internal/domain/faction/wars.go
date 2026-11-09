package faction

import (
	"hash/fnv"
	"slices"
)

// Wars between the factions (EAI-015, original tuning). Which peoples are
// at war is a fixed rule over the generated factions: a hash of their ids,
// types and names (the names come from the world seed), never a draw from
// a random stream, so the same world always has the same wars and
// generating them moves no stream. Wars are mutual and never change in a
// game. Beating an enemy's raid earns its rivals' goodwill once per raid,
// at most once per EnemyCooldownDays each.

const (
	// EnemyBonus is the goodwill a faction gives the colony for driving
	// off a raid of a faction it is at war with.
	EnemyBonus = 6
	// EnemyCooldownDays is how long a faction waits before it thanks the
	// colony for beating its enemies again.
	EnemyCooldownDays = 3
)

// warPct is the chance (percent) that two human factions of types a and b
// are at war: freebooters prey on everyone, settlers and drifters clash
// over the same land, two of a kind seldom fight.
func warPct(a, b Type) int {
	switch {
	case a == Freebooters || b == Freebooters:
		return 70
	case a == b:
		return 10
	case a == Settlers && b == Drifters || a == Drifters && b == Settlers:
		return 50
	}
	return 25
}

// warSeed is the factions' fingerprint the wars are drawn from: FNV-64a
// over each faction's id, type and name.
func warSeed(fs []Faction) uint64 {
	h := fnv.New64a()
	for _, f := range fs {
		_, _ = h.Write([]byte{byte(f.ID), byte(f.ID >> 8), byte(f.Type)})
		_, _ = h.Write([]byte(f.Name))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// warRoll is the pair (a, b)'s roll 0..99 under seed (a < b).
func warRoll(seed uint64, a, b int) int {
	x := seed ^ uint64(a)<<32 ^ uint64(b)
	x ^= x >> 33 // splitmix64 finaliser
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return int(x % 100)
}

// SetEnemies fills every faction's Enemies (ascending ids) from the rule:
// two human factions are at war when their pair's roll falls under
// warPct. It draws no randomness; the same factions always get the same
// wars, so it can be run again (an older save) with the same result.
func SetEnemies(fs []Faction) {
	seed := warSeed(fs)
	for i := range fs {
		fs[i].Enemies = nil
	}
	for i := range fs {
		for j := i + 1; j < len(fs); j++ {
			a, b := &fs[i], &fs[j]
			if !a.Type.Human() || !b.Type.Human() || warRoll(seed, a.ID, b.ID) >= warPct(a.Type, b.Type) {
				continue
			}
			a.Enemies = append(a.Enemies, b.ID)
			b.Enemies = append(b.Enemies, a.ID)
		}
	}
	for i := range fs {
		slices.Sort(fs[i].Enemies)
	}
}

// AtWar reports f at war with faction id.
func (f Faction) AtWar(id int) bool {
	_, ok := slices.BinarySearch(f.Enemies, id)
	return ok
}

// EnemyBonusAt is the goodwill f gives the colony for beating a raid of
// faction id: EnemyBonus when f is at war with id, else 0.
func (f Faction) EnemyBonusAt(id int) int {
	if f.AtWar(id) {
		return EnemyBonus
	}
	return 0
}

// EnemyBeaten is the colony driving off a raid of faction id on colony
// day day: each other faction at war with it, standing and not hostile to
// the colony, thanks it with EnemyBonus goodwill (event names the raid's
// faction), unless it did so within EnemyCooldownDays. It returns the
// indexes of the factions that gave goodwill. The caller calls it once per
// raid.
func EnemyBeaten(fs []Faction, id, day int, event string) []int {
	var out []int
	for i := range fs {
		f := &fs[i]
		if f.ID == id || f.Defeated || f.Relation == Hostile || f.EnemyBonusAt(id) == 0 {
			continue
		}
		if f.ThankedDay > 0 && day+1-f.ThankedDay < EnemyCooldownDays {
			continue // it thanked the colony lately
		}
		f.ThankedDay = day + 1
		Adjust(f, EnemyBonus, event)
		out = append(out, i)
	}
	return out
}

// Rival is the faction (index into fs) that answers for of: one at war
// with fs[of], human, standing and hostile to the colony, picked by roll
// (0..n-1 over the candidates); -1 when fs[of] has no such enemy. A rival
// raid comes from it (EAI-015).
func Rival(fs []Faction, of int, roll func(n int) int) int {
	if of < 0 || of >= len(fs) {
		return -1
	}
	var cands []int
	for _, id := range fs[of].Enemies {
		if i := byID(fs, id); i >= 0 && Raider(fs[i]) {
			cands = append(cands, i)
		}
	}
	switch len(cands) {
	case 0:
		return -1
	case 1:
		return cands[0]
	}
	return cands[roll(len(cands))]
}

// Raider reports f can raid the colony: a standing, hostile human people.
func Raider(f Faction) bool { return f.Type.Human() && !f.Defeated && f.Relation == Hostile }

// byID is the index of the faction with id (-1: none).
func byID(fs []Faction, id int) int {
	if id >= 0 && id < len(fs) && fs[id].ID == id {
		return id // generated factions sit at their id
	}
	return slices.IndexFunc(fs, func(f Faction) bool { return f.ID == id })
}

// EnemyNames is the names of f's enemies among fs, in id order (the
// Factions window).
func EnemyNames(fs []Faction, f Faction) []string {
	var out []string
	for _, id := range f.Enemies {
		if i := byID(fs, id); i >= 0 {
			out = append(out, fs[i].Name)
		}
	}
	return out
}
