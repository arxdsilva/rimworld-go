package weather

import "math"

// sunFull is the share of the sun (permille) each sky lets through at full
// intensity, indexed by Kind. Original tuning (PLAY-022).
var sunFull = [...]int32{Clear: 1000, Cloudy: 700, Rain: 450, Storm: 350, Snow: 400, Blizzard: 250}

// SunFactor is the permille of clear-sky sunlight that reaches the ground
// under sky k at intensity (0..1, clamped): the cover takes half of its
// full-intensity dimming even when light and all of it at full strength.
// Solar output (game/power_gen.go) and the renderer's daylight
// (ebiten/light.go) both read it, so a dim sky and a weak panel agree.
func SunFactor(k Kind, intensity float64) int32 {
	if int(k) >= len(sunFull) {
		return 1000
	}
	dim := float64(1000-sunFull[k]) * (0.5 + 0.5*clamp01(intensity))
	return 1000 - int32(math.Round(dim))
}

// KindOf is the sky named name (Kind.String), Clear for an unknown name:
// read models carry the weather as its name.
func KindOf(name string) Kind {
	for k, n := range names {
		if n == name {
			return Kind(k)
		}
	}
	return Clear
}
