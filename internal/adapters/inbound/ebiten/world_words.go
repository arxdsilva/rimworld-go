package ebiten

import (
	"math/rand/v2"
	"strings"
	"unicode"
)

// The default world seed is a random two-word phrase ("amber-hollow",
// NG-1 §2.6) from these original lists. The wizard runs before any
// simulation exists, so presentation randomness is fine here.

var worldAdjectives = []string{
	"amber", "ashen", "bright", "brisk", "calm", "cedar", "cold", "copper",
	"crimson", "dawn", "dusky", "elder", "ember", "faint", "fallow", "far",
	"fern", "flint", "frost", "gilded", "golden", "grey", "hazel", "high",
	"hollow", "iron", "ivory", "jade", "keen", "lone", "lucky", "mellow",
	"mist", "moss", "northern", "oaken", "pale", "quiet", "red", "rusty",
	"sable", "salt", "silent", "silver", "slate", "stormy", "sunlit", "tawny",
	"umber", "vast", "velvet", "violet", "wild", "willow", "winter", "wren",
}

var worldNouns = []string{
	"anvil", "basin", "beacon", "brook", "canyon", "cinder", "cove", "crown",
	"delta", "dune", "echo", "fjord", "forge", "glade", "harbor", "heath",
	"hollow", "isle", "kettle", "lantern", "marsh", "meadow", "mesa", "orchard",
	"outpost", "pass", "pine", "plume", "quarry", "reach", "ridge", "river",
	"sedge", "shoal", "spire", "spring", "summit", "thicket", "tide", "timber",
	"tundra", "vale", "vista", "wake", "wharf", "wold", "yard", "zenith",
}

// worldPhrase builds "adjective-noun" with n(k) giving a number in [0, k).
func worldPhrase(n func(k int) int) string {
	return worldAdjectives[n(len(worldAdjectives))] + "-" + worldNouns[n(len(worldNouns))]
}

// randomWorldPhrase is a fresh default seed text.
func randomWorldPhrase() string { return worldPhrase(rand.IntN) }

// randomSeed is a fresh non-zero numeric seed (legacy starts).
func randomSeed() uint64 { return rand.Uint64N(1_000_000_000) + 1 }

// Seed text limits (NG-1 §2.6).
const maxSeedText = 32

// seedTextRune reports r is allowed in the world seed: letters, digits,
// space, '-' and '_'.
func seedTextRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_'
}

// normalizeSeedText trims the text and replaces an empty one with a fresh
// phrase, so the seed is always 1..32 characters.
func normalizeSeedText(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxSeedText {
		s = strings.TrimSpace(string(r[:maxSeedText]))
	}
	if s == "" {
		return randomWorldPhrase()
	}
	return s
}
