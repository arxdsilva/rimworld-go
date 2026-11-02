package planet

import (
	"fmt"
	"strings"

	"rimworld-go/pkg/randx"
)

// Original syllables for planet and feature names (WG-09, WG-15): plain
// invented sounds, no borrowed names.
var (
	nameOnsets = []string{"b", "br", "c", "d", "dr", "f", "g", "gr", "h", "k", "l", "m", "n", "p", "r", "s",
		"sh", "st", "t", "th", "tr", "v", "w", "z"}
	nameVowels = []string{"a", "e", "i", "o", "u", "a", "e", "o", "ai", "ou"}
	nameCodas  = []string{"", "", "", "n", "r", "l", "s", "m", "nd", "rr"}
	nameEnds   = []string{"", "", "n", "r", "s", "l", "th", "x", "ra", "na", "ria", "ne", "dor", "ven", "mir", "sa"}

	featurePatterns = [...][]string{
		FeatOcean:     {"the %s Ocean", "the %s Deep"},
		FeatSea:       {"the %s Sea", "the Sea of %s"},
		FeatLake:      {"Lake %s", "%s Mere", "%s Water"},
		FeatBay:       {"%s Bay", "%s Cove", "the %s Inlet"},
		FeatMountains: {"the %s Mountains", "the %s Range", "%s Peaks", "the %s Spine", "the %s Heights"},
		FeatDesert:    {"the %s Waste", "the %s Sands", "%s Barrens", "the %s Dunes"},
		FeatForest:    {"the %s Woods", "%s Forest", "the %s Wilds", "%s Weald"},
		FeatIceCap:    {"the %s Ice", "the %s Icefield", "the %s Glacier"},
	}
)

// nameRoot is a capitalised invented word of syllables syllables.
func nameRoot(r *randx.Rand, syllables int) string {
	var b strings.Builder
	for k := 0; k < syllables; k++ {
		b.WriteString(nameOnsets[r.IntN(len(nameOnsets))])
		b.WriteString(nameVowels[r.IntN(len(nameVowels))])
		if k < syllables-1 {
			b.WriteString(nameCodas[r.IntN(len(nameCodas))])
		}
	}
	b.WriteString(nameEnds[r.IntN(len(nameEnds))])
	s := b.String()
	return strings.ToUpper(s[:1]) + s[1:]
}

// featureName is a fresh name for a kind feature, unique within used.
func featureName(r *randx.Rand, kind FeatureKind, used map[string]bool) string {
	pats := featurePatterns[FeatLake]
	if int(kind) < len(featurePatterns) && len(featurePatterns[kind]) > 0 {
		pats = featurePatterns[kind]
	}
	var name string
	for try := 0; try < 16; try++ {
		name = fmt.Sprintf(pats[r.IntN(len(pats))], nameRoot(r, 2))
		if !used[name] {
			break
		}
	}
	for n := 2; used[name]; n++ {
		name = fmt.Sprintf("%s %d", strings.TrimSuffix(name, fmt.Sprintf(" %d", n-1)), n)
	}
	used[name] = true
	return name
}

// planetName is the planet's invented name.
func planetName(r *randx.Rand) string { return nameRoot(r, 2+r.IntN(2)) }
