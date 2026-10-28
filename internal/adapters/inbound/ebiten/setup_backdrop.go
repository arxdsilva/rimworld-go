package ebiten

import (
	"math"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
	backdropDiscArea  = 192  // offscreen planet render, px
	backdropDiscDist  = 3.4  // camera distance of the disc, radii
	backdropRenderGap = 4    // frames between planet re-renders
	backdropBuildStep = 2000 // showcase generation units per frame
	backdropSeed      = 0x5E7B0A
)

// backdropShowcase is the planet shown when the wizard has none yet.
var backdropShowcase = planet.Params{SeedText: "amber-hollow", Coverage: 100, Rain: 2, Temp: 2, Pop: 1,
	Factions: faction.DefaultSlots()}

// setupBackdropState is the backdrop's cached drawing state. Drawing is
// single-threaded, so one package-level instance serves every App.
type setupBackdropState struct {
	Stars        *ebiten.Image
	StarW, StarH int
	starSeed     uint64

	View    *globeView
	ViewOf  *planet.Planet
	Disc    *ebiten.Image
	discTok int // frame of the last render

	ShowBuilder *planet.Builder
	showPlanet  *planet.Planet
	Plain       *ebiten.Image // shaded sphere while the showcase is generating
	frame       int
}

var (
	setupBackdrop   setupBackdropState
	setupBackdropMu sync.Mutex // parallel tests may draw the backdrop
)

// resetSetupBackdrop drops the cached images (tests).
func resetSetupBackdrop() {
	setupBackdropMu.Lock()
	defer setupBackdropMu.Unlock()
	setupBackdrop = setupBackdropState{}
}

// drawSetupBackdrop draws the setup backdrop for planet p (nil: the
// showcase world).
func (a *App) drawSetupBackdrop(screen *ebiten.Image, sw, sh int, p *planet.Planet) {
	setupBackdropMu.Lock()
	defer setupBackdropMu.Unlock()
	// Full-screen background: physical size, unscaled.
	if bd := screen.Bounds(); bd.Dx() > 0 && bd.Dy() > 0 {
		sw, sh = bd.Dx(), bd.Dy()
	}
	b := &setupBackdrop
	b.frame++
	if !a.drawMenuDiorama(screen, sw, sh, backdropDiorama) {
		screen.Fill(uiSpace)
	}
	reduced := a.settings.Accessibility.ReducedMotion
	t := float64(a.menuTick)
	if reduced {
		t = 0
	}
	b.drawStars(screen, sw, sh, p, t)
	b.drawPlanet(screen, sw, sh, p, t, reduced)
}

func (b *setupBackdropState) drawStars(screen *ebiten.Image, sw, sh int, p *planet.Planet, t float64) {
	seed := uint64(backdropSeed)
	if p != nil {
		seed = p.Seed
	}
	if b.Stars == nil || b.StarW != sw || b.StarH != sh || b.starSeed != seed {
		b.Stars = ebiten.NewImageFromImage(bakeStarfield(seed, sw, sh, 0.9, 0.6))
		b.StarW, b.StarH, b.starSeed = sw, sh, seed
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(6*math.Sin(t/700), 3*math.Sin(t/1100+1))
	screen.DrawImage(b.Stars, op)
}

// DiscPlanet is the planet to render: p, or the showcase once generated
// (stepping its generation a little each call).
func (b *setupBackdropState) DiscPlanet(p *planet.Planet) *planet.Planet {
	if p != nil {
		return p
	}
	if b.showPlanet == nil {
		if b.ShowBuilder == nil {
			b.ShowBuilder = planet.NewBuilder(backdropShowcase)
		}
		if _, _, done := b.ShowBuilder.Step(backdropBuildStep); done {
			b.showPlanet = b.ShowBuilder.Planet()
			b.ShowBuilder = nil
		}
	}
	return b.showPlanet
}

func (b *setupBackdropState) drawPlanet(screen *ebiten.Image, sw, sh int, p *planet.Planet, t float64, reduced bool) {
	// Placement: upper right, drifting a few pixels.
	r := math.Min(float64(sh)*0.15, 130)
	cx := float64(sw) * 0.83
	cy := float64(sh) * 0.22
	if !reduced {
		cx += 8 * math.Sin(t/900)
		cy += 5 * math.Sin(t/1300+2)
	}
	scale := r / discSilhouette()

	src := b.DiscPlanet(p)
	if src == nil {
		if b.Plain == nil {
			b.Plain = ebiten.NewImageFromImage(bakeSphere(globeSphereBakeSize, globeSphereBakeR,
				palRGB("ocean_deep"), palRGB("atmosphere_rim")))
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		k := r / globeSphereBakeR
		op.GeoM.Translate(-globeSphereBakeSize/2, -globeSphereBakeSize/2)
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(cx, cy)
		screen.DrawImage(b.Plain, op)
		return
	}
	if b.View == nil || b.ViewOf != src {
		g := newGlobeView(src, nil)
		g.Rivers, g.Roads, g.Marks = nil, nil, nil // far away: the bare tiles
		g.SetAreaPx(0, 0, backdropDiscArea, backdropDiscArea)
		g.Cam.SetDist(backdropDiscDist)
		g.Cam.Lat, g.Cam.Lon = 0.35, float64(src.Seed%628)/100
		b.View, b.ViewOf = g, src
		b.Disc = ebiten.NewImage(backdropDiscArea, backdropDiscArea)
		b.discTok = -backdropRenderGap
	}
	if b.frame-b.discTok >= backdropRenderGap && (!reduced || b.discTok < 0) {
		g := b.View
		if !reduced {
			g.Cam.Lon += 0.0016
			g.Cam.Lat = 0.35 + 0.12*math.Sin(t/1500)
		}
		g.EnsureGPU(backdropDiscArea, backdropDiscArea)
		b.Disc.Clear()
		g.DrawBody(b.Disc)
		b.discTok = b.frame
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-backdropDiscArea/2, -backdropDiscArea/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(cx, cy)
	screen.DrawImage(b.Disc, op)
}

// discSilhouette is the disc render's silhouette radius in px.
func discSilhouette() float64 {
	return 1.2 * backdropDiscArea / math.Sqrt(backdropDiscDist*backdropDiscDist-1)
}
