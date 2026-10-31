package farm

import (
	"strconv"
	"strings"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Cause is what killed a plant in the growth pass.
type Cause uint8

const (
	Cold     Cause = iota // frost (a crop) or cold (a wild plant)
	Heat                  // heat (a wild plant)
	Rot                   // FOOD-2: outlived its lifespan (wiki: "died from rotting")
	Starved               // FOOD-2: an unpowered hydroponics basin
	Withered              // too much light (duskmoss): its own notice, per patch
	Toxic                 // FOOD-6: toxic fallout (the event's letter warned; no notice)
)

// Kill is one species' deaths of one cause in a growth pass (a notice
// each), at the first plant's spot.
type Kill struct {
	Species plant.Species
	N       int
	X, Y    int
	Cause   Cause
}

// Pass collects one growth pass's deaths: the ids to drop from the plant
// list, the notices to post, and the species frost or heat began to hurt
// (a warning each, at the first such plant).
type Pass struct {
	Gone  map[entity.EntityID]bool
	Kills []Kill
	Hurt  []Kill
}

// Died records plant id (p) dead of cause.
func (fp *Pass) Died(id entity.EntityID, p plant.Plant, cause Cause) {
	if fp.Gone == nil {
		fp.Gone = map[entity.EntityID]bool{}
	}
	fp.Gone[id] = true
	if cause != Withered && cause != Toxic {
		fp.Kills = Add(fp.Kills, p, cause)
	}
}

// DamageCause is the cause of p's cold or heat damage.
func DamageCause(p plant.Plant) Cause {
	if p.DamageHeat {
		return Heat
	}
	return Cold
}

// Add counts plant p in ks: one entry per species and cause.
func Add(ks []Kill, p plant.Plant, cause Cause) []Kill {
	for i := range ks {
		if ks[i].Species == p.Species && ks[i].Cause == cause {
			ks[i].N++
			return ks
		}
	}
	return append(ks, Kill{Species: p.Species, N: 1, X: p.X, Y: p.Y, Cause: cause})
}

// HurtText is the warning that frost (or heat, hot) is hurting species
// s: "Frost is damaging the corn", "Heat is scorching the pine plants".
func HurtText(s plant.Species, hot bool) string {
	name := strings.ReplaceAll(s.DisplayName(), "_", " ") + " plants"
	if plant.DefOf(s).Crop {
		name = CropField(s)
	}
	if hot {
		return "Heat is scorching the " + name
	}
	return "Frost is damaging the " + name
}

// CropField is a crop's name as a field of it ("the potatoes").
func CropField(s plant.Species) string {
	switch s {
	case plant.Potato:
		return "potatoes"
	case plant.Radish:
		return "radishes"
	case plant.Pumpkin:
		return "pumpkins"
	case plant.Strawberry:
		return "strawberries"
	}
	return s.String()
}

// KillText is the notice for n plants of species s dead of cause.
func KillText(s plant.Species, n int, cause Cause) string {
	d := plant.DefOf(s)
	switch {
	case cause == Rot && n == 1:
		return "A " + s.String() + " plant died of old age"
	case cause == Rot:
		return "The " + CropField(s) + " died of old age (" + strconv.Itoa(n) + " plants)"
	case cause == Starved && n == 1:
		return "A " + s.String() + " plant withered in an unpowered hydroponics basin"
	case cause == Starved:
		return "The " + CropField(s) + " withered in unpowered hydroponics basins (" + strconv.Itoa(n) + " plants)"
	case d.Crop && n == 1:
		return "Frost killed a " + s.String() + " plant"
	case d.Crop:
		return "Frost killed the " + d.Label + " (" + strconv.Itoa(n) + " plants)"
	}
	what := "cold"
	if cause == Heat {
		what = "heat"
	}
	name := strings.ReplaceAll(s.DisplayName(), "_", " ")
	if n == 1 {
		return "The " + what + " killed a " + name
	}
	return "The " + what + " killed " + strconv.Itoa(n) + " " + name + " plants"
}

// KillCount is the plant count of a notice KillText wrote for s and
// cause (false if text is not one).
func KillCount(text string, s plant.Species, cause Cause) (int, bool) {
	if text == KillText(s, 1, cause) {
		return 1, true
	}
	const mark = 987654321 // a count no notice holds: splits the plural text
	pre, post, _ := strings.Cut(KillText(s, mark, cause), strconv.Itoa(mark))
	if !strings.HasPrefix(text, pre) || !strings.HasSuffix(text, post) || len(text) <= len(pre)+len(post) {
		return 0, false
	}
	n, err := strconv.Atoi(text[len(pre) : len(text)-len(post)])
	if err != nil || n < 2 {
		return 0, false
	}
	return n, true
}

// WitherText is the notice for a plant of species s withered by light.
func WitherText(s plant.Species) string {
	name := strings.ReplaceAll(s.DisplayName(), "_", " ")
	if name == "" {
		return "A plant withered in the light"
	}
	return strings.ToUpper(name[:1]) + name[1:] + " withered in the light"
}
