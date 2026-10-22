package faction

import "strings"

// blocklist is the NG-1 originality lint (§0.4, §3): proper nouns of the
// reference game that no storyteller, scenario, faction or generated name
// may use. Phrases match whole words in order, case-insensitively, so
// "randy" blocks "Randy Vane" but not "Brandy".
var blocklist = []string{
	// storytellers
	"cassandra", "phoebe", "randy", "cassandra classic", "phoebe chillax", "randy random",
	// scenarios
	"crashlanded", "crash landed", "lost tribe", "rich explorer", "naked brutality", "new arrivals", "new tribe",
	"the rich explorer", "tribal", "tribe", "tribes",
	// factions and their stock names
	"outlander", "outlanders", "outlander union", "rough outlander union", "pirate", "pirates", "pirate gang",
	"pirate band", "pillager", "pillagers", "waster", "wasters", "empire", "mechanoid", "mechanoids",
	"insectoid", "insectoids", "ancients", "savage tribe", "rough tribe", "gentle tribe", "fierce tribe",
	// creatures and the game itself
	"thrumbo", "muffalo", "megasloth", "boomalope", "boomrat", "alphabeaver", "megascarab", "rimworld",
}

// Blocklist lists the blocked phrases (for tools such as contentlint).
func Blocklist() []string { return append([]string(nil), blocklist...) }

// words lower-cases s and splits it into runs of letters.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && r < 0x80 || r == '\''
	})
}

// Blocked reports the first blocklisted phrase found in text.
func Blocked(text string) (phrase string, bad bool) {
	tw := words(text)
	for _, p := range blocklist {
		pw := words(p)
	scan:
		for i := 0; i+len(pw) <= len(tw); i++ {
			for j, w := range pw {
				if tw[i+j] != w {
					continue scan
				}
			}
			return p, true
		}
	}
	return "", false
}
