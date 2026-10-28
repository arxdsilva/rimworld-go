package wizard

import (
	"math"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// The backdrop behind every setup screen except the landing screen (NG-1
// §1.3): the dimmed menu diorama, a baked starfield and a small distant
// planet disc rendered from the generated world (or, before one exists, a
// showcase world that is generated a few steps per frame). Everything
// drifts slowly; Reduced Motion freezes it.

const (
	backdropDiorama   = 0.55 // diorama dim
	BackdropDiscArea  = 192  // offscreen planet render, px
	BackdropDiscDist  = 3.4  // camera distance of the disc, radii
	BackdropRenderGap = 4    // frames between planet re-renders
	backdropBuildStep = 2000 // showcase generation units per frame
	backdropSeed      = 0x5E7B0A
)

// BackdropShowcase is the planet shown when the wizard has none yet.
var BackdropShowcase = planet.Params{SeedText: "amber-hollow", Coverage: 100, Rain: 2, Temp: 2, Pop: 1,
	Factions: faction.DefaultSlots()}

// BackdropState is the backdrop's cached drawing state. Drawing is
// single-threaded, so one package-level instance serves every Flow.
type BackdropState struct {
	Stars        *ebiten.Image
	StarW, StarH int
	starSeed     uint64

	View    *globe.View
	ViewOf  *planet.Planet
	Disc    *ebiten.Image
	discTok int // frame of the last render

	ShowBuilder *planet.Builder
	showPlanet  *planet.Planet
	Plain       *ebiten.Image // shaded sphere while the showcase is generating
	frame       int
}

var (
	Backdrop   BackdropState
	BackdropMu sync.Mutex // parallel tests may draw the backdrop
)

// ResetBackdrop drops the cached images (tests).
func ResetBackdrop() {
	BackdropMu.Lock()
	defer BackdropMu.Unlock()
	Backdrop = BackdropState{}
}

// DrawBackdrop draws the setup backdrop for planet p (nil: the
// showcase world).
func (w *Flow) DrawBackdrop(screen *ebiten.Image, sw, sh int, p *planet.Planet) {
	BackdropMu.Lock()
	defer BackdropMu.Unlock()
	// Full-screen background: physical size, unscaled.
	if bd := screen.Bounds(); bd.Dx() > 0 && bd.Dy() > 0 {
		sw, sh = bd.Dx(), bd.Dy()
	}
	b := &Backdrop
	b.frame++
	if !w.sh.DrawDiorama(screen, sw, sh, backdropDiorama) {
		screen.Fill(uikit.Space)
	}
	reduced := w.sh.Settings().Accessibility.ReducedMotion
	t := float64(w.sh.Env().MenuTick)
	if reduced {
		t = 0
	}
	b.drawStars(screen, sw, sh, p, t)
	b.drawPlanet(screen, sw, sh, p, t, reduced)
}

func (b *BackdropState) drawStars(screen *ebiten.Image, sw, sh int, p *planet.Planet, t float64) {
	seed := uint64(backdropSeed)
	if p != nil {
		seed = p.Seed
	}
	if b.Stars == nil || b.StarW != sw || b.StarH != sh || b.starSeed != seed {
		b.Stars = ebiten.NewImageFromImage(globe.BakeStarfield(seed, sw, sh, 0.9, 0.6))
		b.StarW, b.StarH, b.starSeed = sw, sh, seed
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(6*math.Sin(t/700), 3*math.Sin(t/1100+1))
	screen.DrawImage(b.Stars, op)
}

// DiscPlanet is the planet to render: p, or the showcase once generated
// (stepping its generation a little each call).
func (b *BackdropState) DiscPlanet(p *planet.Planet) *planet.Planet {
	if p != nil {
		return p
	}
	if b.showPlanet == nil {
		if b.ShowBuilder == nil {
			b.ShowBuilder = planet.NewBuilder(BackdropShowcase)
		}
		if _, _, done := b.ShowBuilder.Step(backdropBuildStep); done {
			b.showPlanet = b.ShowBuilder.Planet()
			b.ShowBuilder = nil
		}
	}
	return b.showPlanet
}

func (b *BackdropState) drawPlanet(screen *ebiten.Image, sw, sh int, p *planet.Planet, t float64, reduced bool) {
	// Placement: upper right, drifting a few pixels.
	r := math.Min(float64(sh)*0.15, 130)
	cx := float64(sw) * 0.83
	cy := float64(sh) * 0.22
	if !reduced {
		cx += 8 * math.Sin(t/900)
		cy += 5 * math.Sin(t/1300+2)
	}
	scale := r / DiscSilhouette()

	src := b.DiscPlanet(p)
	if src == nil {
		if b.Plain == nil {
			b.Plain = ebiten.NewImageFromImage(globe.BakeSphere(globe.SphereBakeSize, globe.SphereBakeR,
				globe.PalRGB("ocean_deep"), globe.PalRGB("atmosphere_rim")))
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		k := r / globe.SphereBakeR
		op.GeoM.Translate(-globe.SphereBakeSize/2, -globe.SphereBakeSize/2)
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(cx, cy)
		screen.DrawImage(b.Plain, op)
		return
	}
	if b.View == nil || b.ViewOf != src {
		g := globe.New(src, nil)
		g.Rivers, g.Roads, g.Marks = nil, nil, nil // far away: the bare tiles
		g.SetAreaPx(0, 0, BackdropDiscArea, BackdropDiscArea)
		g.Cam.SetDist(BackdropDiscDist)
		g.Cam.Lat, g.Cam.Lon = 0.35, float64(src.Seed%628)/100
		b.View, b.ViewOf = g, src
		b.Disc = ebiten.NewImage(BackdropDiscArea, BackdropDiscArea)
		b.discTok = -BackdropRenderGap
	}
	if b.frame-b.discTok >= BackdropRenderGap && (!reduced || b.discTok < 0) {
		g := b.View
		if !reduced {
			g.Cam.Lon += 0.0016
			g.Cam.Lat = 0.35 + 0.12*math.Sin(t/1500)
		}
		g.EnsureGPU(BackdropDiscArea, BackdropDiscArea)
		b.Disc.Clear()
		g.DrawBody(b.Disc)
		b.discTok = b.frame
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-BackdropDiscArea/2, -BackdropDiscArea/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(cx, cy)
	screen.DrawImage(b.Disc, op)
}

// DiscSilhouette is the disc render's silhouette radius in px.
func DiscSilhouette() float64 {
	return 1.2 * BackdropDiscArea / math.Sqrt(BackdropDiscDist*BackdropDiscDist-1)
}
