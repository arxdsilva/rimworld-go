package render

// Paths is a bitset of render migrations. Each bit selects one migration's
// new draw path; the zero value is the old path everywhere. The pixel-diff
// harness (-capture-diff, render/pixeldiff) renders a scene once with no bit
// set and once with the bits under test, and compares the two. The worker
// that lands a migration removes its bit in the same change.
type Paths uint32

const (
	// PathsTest is used only by the harness's own self-test: the new path
	// alters one pixel so the report has something to find.
	PathsTest Paths = 1 << iota
)

// Has reports whether every bit of b is set.
func (p Paths) Has(b Paths) bool { return p&b == b && b != 0 }

// ParsePaths reads a comma separated list of path names ("test"); "" is none.
func ParsePaths(s string) (Paths, bool) {
	var p Paths
	for _, n := range splitComma(s) {
		switch n {
		case "test":
			p |= PathsTest
		default:
			return 0, false
		}
	}
	return p, true
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}
