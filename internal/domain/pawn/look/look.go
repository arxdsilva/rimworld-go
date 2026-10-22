// Package look is a colonist's appearance as palette indices (no pixels):
// sex, build, skin, hair, beard and clothes (NG-1 §4.6, ART §10).
package look

// Sex drives names, pronouns and art; no stat differences.
type Sex uint8

const (
	Male Sex = iota
	Female
)

// Build is the body shape (art build and a few stats).
type Build uint8

const (
	Average Build = iota
	Thin
	Heavy
)

// Look is everything the renderer needs to draw a pawn.
type Look struct {
	Sex       Sex
	Build     Build
	Skin      uint8 // index into the skin palette
	Hair      uint8 // index into the hair palette
	HairStyle uint8 // index into HairStyles
	Beard     uint8 // 0 none, then index into Beards+1
	Shirt     uint8
	Pants     uint8
	Vest      bool
	Grey      bool // greying hair (older pawns)
}

// HairStyles and Beards name the art variants (keys in the pawn family).
var (
	HairStyles = []string{"short", "crop", "bun", "long", "braid", "shaved", "curls", "tail"}
	Beards     = []string{"short", "full", "moustache"}
)

// BuildKey is the art build prefix: m_avg, f_thin, ...
func (l Look) BuildKey() string { return "m_avg" } // NG-1 stub (PAWN-D)

// Legacy reproduces today's renderer look for pawn id exactly (male,
// average build, short/crop/bun hair), given the palette sizes the
// renderer used (skins, hairs, cloths).
func Legacy(id uint64, skins, hairs, cloths int) Look { return Look{} } // NG-1 stub (PAWN-D)
