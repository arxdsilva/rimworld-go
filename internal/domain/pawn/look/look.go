// Package look is a colonist's appearance as palette indices (no pixels):
// sex, build, skin, hair, beard and clothes (NG-1 §4.6, ART §10).
//
// Palette indices refer to the renderer's tint groups sorted by name
// ("skin", "hair", and "cloth" without the raider tint). A renderer
// resolves index i as palette[i % len(palette)], so indices chosen for a
// larger palette still draw if a palette shrinks.
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
	Grey      bool  // greying hair (older pawns)
	VestCloth uint8 // cloth index of the vest when Vest is set
	Bare      bool  // arrived without clothes: no shirt, pants or vest
}

// HairStyles and Beards name the art variants (keys in the pawn family).
var (
	HairStyles = []string{"short", "crop", "bun", "long", "braid", "shaved", "curls", "tail"}
	Beards     = []string{"short", "full", "moustache"}
)

// Palette sizes the generator draws from: the sorted tint groups in
// assets/palettes/base.yaml (cloth without the raider tint).
const (
	SkinCount  = 5
	HairCount  = 5
	ClothCount = 6
)

// legacyHairStyles is how many of HairStyles the pre-NG-1 renderer used
// (short, crop, bun, in that order).
const legacyHairStyles = 3

// String names a sex in lower case.
func (s Sex) String() string {
	if s == Female {
		return "female"
	}
	return "male"
}

// String names a build in lower case.
func (b Build) String() string {
	switch b {
	case Thin:
		return "thin"
	case Heavy:
		return "heavy"
	}
	return "average"
}

// BuildKey is the art build prefix: m_avg, f_thin, ...
func (l Look) BuildKey() string {
	p := "m_"
	if l.Sex == Female {
		p = "f_"
	}
	switch l.Build {
	case Thin:
		return p + "thin"
	case Heavy:
		return p + "heavy"
	}
	return p + "avg"
}

// mix64 is the splitmix64 finaliser the renderer has always used.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return x
}

// Legacy reproduces today's renderer look for pawn id exactly (male,
// average build, short/crop/bun hair), given the palette sizes the
// renderer used (skins, hairs, cloths). Palette names are unique (they
// are map keys), so comparing indices equals comparing names.
func Legacy(id uint64, skins, hairs, cloths int) Look {
	pick := func(salt uint64, n int) uint8 {
		if n <= 0 {
			return 0
		}
		return uint8(mix64(id*0x9E3779B97F4A7C15+salt) % uint64(n))
	}
	l := Look{
		Sex:       Male,
		Build:     Average,
		Skin:      pick(1, skins),
		Hair:      pick(2, hairs),
		HairStyle: pick(3, legacyHairStyles),
		Shirt:     pick(4, cloths),
		Pants:     pick(5, cloths),
	}
	if mix64(id+6)%2 == 0 {
		l.Vest = cloths > 0 // an empty palette gave the renderer "" (no vest)
		l.VestCloth = pick(7, cloths)
	}
	if l.Pants == l.Shirt && cloths > 1 {
		// the renderer takes the first cloth that differs from the shirt
		if l.Shirt == 0 {
			l.Pants = 1
		} else {
			l.Pants = 0
		}
	}
	return l
}
