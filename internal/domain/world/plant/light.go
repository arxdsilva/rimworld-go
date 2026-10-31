package plant

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/terrain"
)

// Light rules (MAPGEN-3). Light is in permille of full daylight (the
// simulation's cell light × 1000), compared against the Def's percents.

// StrongGrowLight is the artificial light (0..1) that regrows grass at
// night (PLAY-017): a lamp within two tiles, or a burning campfire beside
// it. Plants rest at night whatever the light (FOOD-2, farm.go).
const StrongGrowLight = 0.5

// strongLightEps lets a light rounded through the float32 light grid
// still count at exactly StrongGrowLight.
const strongLightEps = 1e-6

// StrongLight reports artificial light l (0..1) strong enough to grow a
// plant round the clock.
func StrongLight(l float64) bool { return l >= StrongGrowLight-strongLightEps }

// OwnLightRules reports a species with light rules of its own (ordinary
// moss needs light, duskmoss withers in it and grows in caves): it keeps
// them and the plain GrowSpan, and the sun-or-lamp rule leaves it alone.
func OwnLightRules(s Species) bool {
	d := DefOf(s)
	return d.MinLight > 0 || d.DiesInLight > 0 || d.Cave
}

// WitherStopLight (percent) is the light a withering plant must fall
// below to stop withering: harm starts at DiesInLight and stops only
// under this, so a light flickering at the threshold doesn't flap.
const WitherStopLight = 35

// witherPerHour is the share (percent) of its full growth a plant loses
// per in-game hour in harmful light.
const witherPerHour = 8

// Fertility is the fertility (percent) species s grows at on ground t:
// the soil's, or CaveFertility for a Cave species on rough or smoothed
// stone (other species get nothing from stone).
func Fertility(s Species, t terrain.Type) int {
	if f := t.Fertility(); f > 0 {
		return f
	}
	if DefOf(s).Cave && (t == terrain.Stone || t == terrain.StoneSmooth) {
		return CaveFertility
	}
	return 0
}

// LitEnough reports whether species s may grow at light (permille).
func LitEnough(s Species, light int) bool {
	return light >= DefOf(s).MinLight*10
}

// Harmed reports whether a plant of species s withers at light
// (permille): from DiesInLight up, and once withering, until the light
// falls below WitherStopLight (hysteresis).
func Harmed(s Species, withering bool, light int) bool {
	d := DefOf(s)
	if d.DiesInLight <= 0 {
		return false
	}
	if withering {
		return light >= min(WitherStopLight, d.DiesInLight)*10
	}
	return light >= d.DiesInLight*10
}

// WitherLoss is the growth (ticks) a plant of species s loses over ticks
// of harmful light: witherPerHour of its full growth an hour, at least 1.
func WitherLoss(s Species, ticks int) int {
	d := DefOf(s)
	if d.DiesInLight <= 0 || ticks <= 0 {
		return 0
	}
	return max(1, d.GrowTicks*witherPerHour*ticks/(100*clock.TicksPerHour))
}
