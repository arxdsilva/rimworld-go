package ebiten

import (
	"image"
	"image/color"

	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// The wizard's planet view lives in globe (ARCH-003). These aliases and
// one-line wrappers keep the root's call sites unchanged while its features
// move out one package at a time (SOLID_GO.md §11: no forwarder outlives the
// commit series that repoints its callers).

type (
	globeView  = globe.View
	globeInput = globe.Input
	globeMode  = globe.Mode
	colourStop = globe.ColourStop
)

const (
	globeBiome          = globe.ModeBiome
	globeTemperature    = globe.ModeTemperature
	globeRainfall       = globe.ModeRainfall
	globeElevation      = globe.ModeElevation
	globeNone           = globe.ModeNone
	globeUncovered      = globe.ModeUncovered
	numGlobeModes       = globe.NumModes
	globeCullSlack      = globe.CullSlack
	globeDoubleFrames   = globe.DoubleFrames
	globeFlyFrames      = globe.FlyFrames
	globeSphereBakeR    = globe.SphereBakeR
	globeSphereBakeSize = globe.SphereBakeSize
)

func newGlobeView(p *planet.Planet, cat *artCatalog) *globeView { return globe.New(p, cat) }

// globeInputFrom adapts the App's frame: w x h is the logical screen, the
// pointer the raw (physical) one the globe picks and drags with. blocked
// is whether the pointer is over screen chrome.
func globeInputFrom(in *inputFrame, w, h int, blocked bool) globeInput {
	return globeInput{
		W: w, H: h, MX: in.Raw.Mx, MY: in.Raw.My, Wheel: in.Raw.WheelY,
		Down: in.Raw.Down, Press: in.Raw.Press, Release: in.Raw.Release,
		Held: in.Raw.Held, Keys: in.Raw.Pressed, Shift: in.Shift(), Blocked: blocked,
	}
}

var (
	tempRamp = globe.TempRamp
	rainRamp = globe.RainRamp
	elevRamp = globe.ElevRamp
)

func palRGB(key string) [3]float32                  { return globe.PalRGB(key) }
func ramp(stops []colourStop, v float32) [3]float32 { return globe.Ramp(stops, v) }
func globeAccent(t faction.Type) color.NRGBA        { return globe.Accent(t) }
func globeClamp(v, lo, hi float64) float64          { return globe.Clamp(v, lo, hi) }

func bakeSphere(size, r int, uncovered, rimC [3]float32) *image.NRGBA {
	return globe.BakeSphere(size, r, uncovered, rimC)
}

func bakeStarfield(seed uint64, w, h int, strength, skyFade float64) *image.NRGBA {
	return globe.BakeStarfield(seed, w, h, strength, skyFade)
}
