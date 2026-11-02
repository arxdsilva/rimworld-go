package construction

// Carpet floor colours (AX-3). A carpet floor's Building.Tint and
// Frame.Tint are an index into CarpetColours (0, the first, for a carpet
// laid before colours existed). The set follows vanilla's 63 carpet colours
// one for one (wiki: Carpet, Fine carpet), under names of our own (ENG-003).
// The colour costs nothing: every carpet floor is 1 cloth. Saved by index:
// append only, never reorder. The renderer draws each by name from the
// `carpet` tints of assets/palettes/base.yaml.
var CarpetColours = [...]string{
	"crimson", "rosewood", "rust", "poppy", "wine", "damson", "heather", "fuchsia", "orchid",
	"violet", "lilac", "amethyst", "aubergine", "mulberry", "midnight", "periwinkle", "cobalt",
	"denim", "frost", "navy", "cornflower", "twilight", "lichen", "fern", "lagoon", "clover",
	"eucalyptus", "celadon", "spruce", "juniper", "bog", "reed", "rush", "emerald", "seafoam",
	"laurel", "thicket", "ochre", "pistachio", "taupe", "fawn", "caramel", "chocolate", "loam",
	"sienna", "chestnut", "tangerine", "blush", "rose", "powder blue", "mint", "butter",
	"apricot", "snow", "ivory", "ash", "chalk", "pearl", "clay", "flint", "graphite", "storm",
	"onyx",
}

// TintName is the colour of a kind-k piece of tint (a carpet floor's
// colour name; "" for every other kind).
func TintName(k Kind, tint uint8) string {
	if k != CarpetFloor {
		return ""
	}
	if int(tint) >= len(CarpetColours) {
		tint = 0
	}
	return CarpetColours[tint]
}

// ParseColour is the index of carpet colour name.
func ParseColour(name string) (uint8, bool) {
	for i, n := range CarpetColours {
		if n == name {
			return uint8(i), true
		}
	}
	return 0, false
}

// TintFrame gives blueprint f (nil: none) carpet colour index i: only a
// carpet floor takes one, and an index off the list leaves it as it is
// (DesignateBuild carries the colour in Command.Index).
func TintFrame(f *Frame, i int) {
	if f != nil && f.Kind == CarpetFloor && i >= 0 && i < len(CarpetColours) {
		f.Tint = uint8(i)
	}
}
