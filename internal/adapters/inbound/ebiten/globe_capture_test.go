package ebiten

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// TestGlobeCapture writes review PNGs of the globe when GLOBE_CAPTURE_DIR
// is set (a manual check; it is skipped otherwise). A GPU is not
// available to tests, so it rasterises the frame's meshes on the CPU
// (Gouraud triangles, alpha overlays, the baked sphere and starfield, and
// the real emblem sprites): the geometry, colours and placement are the
// ones the GPU draws; only anti-aliasing and text differ.
func TestGlobeCapture(t *testing.T) {
	dir := os.Getenv("GLOBE_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set GLOBE_CAPTURE_DIR to write globe captures")
	}
	root, err := LocateArt()
	var art *artCatalog
	if err == nil {
		art, _ = loadArtCatalog(root)
	}
	const W, H = 1280, 720
	shot := func(name string, g *globeView) {
		g.sw, g.sh = W, H
		img := image.NewRGBA(image.Rect(0, 0, W, H))
		draw.Draw(img, img.Bounds(), image.NewUniform(uiSpace), image.Point{}, draw.Src)
		drawOver(img, bakeStarfield(g.p.Seed, W, H, 1, 0), 0, 0, 1)
		vp := g.viewport()
		r := globemath.SilhouetteRadius(vp, g.cam.Dist)
		sph := bakeSphere(globeSphereBakeSize, globeSphereBakeR, palRGB("uncovered"), palRGB("atmosphere_rim"))
		sc := r / globeSphereBakeR
		for y := int(vp.CY - 1.3*r); y <= int(vp.CY+1.3*r); y++ {
			for x := int(vp.CX - 1.3*r); x <= int(vp.CX+1.3*r); x++ {
				if x < 0 || y < 0 || x >= W || y >= H {
					continue
				}
				sx := int((float64(x)-vp.CX)/sc) + globeSphereBakeSize/2
				sy := int((float64(y)-vp.CY)/sc) + globeSphereBakeSize/2
				if sx < 0 || sy < 0 || sx >= globeSphereBakeSize || sy >= globeSphereBakeSize {
					continue
				}
				blend(img, x, y, sph.NRGBAAt(sx, sy), 1)
			}
		}
		g.build()
		raster(img, g.verts, g.idx[:g.nI])
		raster(img, g.ov, g.oi)
		size := globeClamp(g.tilePx()*2, 12, 46)
		for k := range g.marks {
			m := &g.marks[k]
			sx, sy, vz := g.worldPt(&g.p.Center[m.tile])
			if vz < g.zh+0.04 {
				continue
			}
			disc(img, float64(sx), float64(sy), size*0.56, color.NRGBA{10, 14, 20, 150})
			if sp := art.sprite(m.key); sp != nil {
				pix := sp.pix
				scale := size / float64(pix.Bounds().Dx())
				for y := 0; y < int(size); y++ {
					for x := 0; x < int(size); x++ {
						c := pix.NRGBAAt(int(float64(x)/scale), int(float64(y)/scale))
						c = color.NRGBA{uint8(int(c.R) * int(m.tint.R) / 255), uint8(int(c.G) * int(m.tint.G) / 255),
							uint8(int(c.B) * int(m.tint.B) / 255), c.A}
						blend(img, int(float64(sx)-size/2)+x, int(float64(sy)-size/2)+y, c, 1)
					}
				}
			} else {
				disc(img, float64(sx), float64(sy), size*0.32, m.tint)
			}
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	for _, cov := range []int{30, 100} {
		p := planet.Build(planet.Params{SeedText: "amber-hollow", Coverage: cov, Rain: 2, Temp: 2, Pop: 1,
			Factions: faction.DefaultSlots()})
		g := newGlobeView(p, art)
		g.reduced = true
		tag := map[int]string{30: "30", 100: "100"}[cov]
		g.cam.Lat, g.cam.Lon = 0.3, 0.5
		if cov == 30 {
			c := p.Pole
			centreOn(g, [3]float32{c[0], c[1], c[2]})
		}
		g.Select(landTile(t, g))
		g.hovered = int(p.Neighbours(g.selected)[0])
		shot("globe_"+tag+"_biome.png", g)
		g.hovered = -1
		g.cam.SetDist(1.45)
		if len(g.marks) > 0 {
			g.Center(int(g.marks[0].tile))
		}
		shot("globe_"+tag+"_zoom.png", g)
		g.cam.SetDist(2.9)
		for _, m := range []globeMode{globeTemperature, globeRainfall, globeElevation} {
			g.SetMode(m)
			shot("globe_"+tag+"_"+m.String()+".png", g)
		}
	}
	// The backdrop's planet disc: stars plus the disc as it is composed
	// (the diorama is not part of this capture).
	{
		p := planet.Build(backdropShowcase)
		g := newGlobeView(p, nil)
		g.rivers, g.roads, g.marks = nil, nil, nil
		g.SetArea(0, 0, backdropDiscArea, backdropDiscArea)
		g.cam.SetDist(backdropDiscDist)
		g.cam.Lat, g.cam.Lon = 0.35, float64(p.Seed%628)/100
		g.sw, g.sh = backdropDiscArea, backdropDiscArea
		disc := image.NewRGBA(image.Rect(0, 0, backdropDiscArea, backdropDiscArea))
		sph := bakeSphere(globeSphereBakeSize, globeSphereBakeR, palRGB("uncovered"), palRGB("atmosphere_rim"))
		vp := g.viewport()
		r := globemath.SilhouetteRadius(vp, g.cam.Dist)
		sc := r / globeSphereBakeR
		for y := 0; y < backdropDiscArea; y++ {
			for x := 0; x < backdropDiscArea; x++ {
				sx := int((float64(x)-vp.CX)/sc) + globeSphereBakeSize/2
				sy := int((float64(y)-vp.CY)/sc) + globeSphereBakeSize/2
				if sx >= 0 && sy >= 0 && sx < globeSphereBakeSize && sy < globeSphereBakeSize {
					c := sph.NRGBAAt(sx, sy)
					disc.SetRGBA(x, y, color.RGBA{uint8(int(c.R) * int(c.A) / 255), uint8(int(c.G) * int(c.A) / 255), uint8(int(c.B) * int(c.A) / 255), c.A})
				}
			}
		}
		g.build()
		raster(disc, g.verts, g.idx[:g.nI])
		bg := image.NewRGBA(image.Rect(0, 0, W, H))
		draw.Draw(bg, bg.Bounds(), image.NewUniform(uiSpace), image.Point{}, draw.Src)
		drawOver(bg, bakeStarfield(p.Seed, W, H, 0.9, 0.6), 0, 0, 1)
		rr := math.Min(float64(H)*0.15, 130)
		scale := rr / discSilhouette()
		cx, cy := float64(W)*0.83, float64(H)*0.22
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				sx := int((float64(x)-cx)/scale + backdropDiscArea/2)
				sy := int((float64(y)-cy)/scale + backdropDiscArea/2)
				if sx >= 0 && sy >= 0 && sx < backdropDiscArea && sy < backdropDiscArea {
					c := disc.RGBAAt(sx, sy)
					blend(bg, x, y, color.NRGBA{c.R, c.G, c.B, 255}, float64(c.A)/255)
				}
			}
		}
		f, err := os.Create(filepath.Join(dir, "backdrop_disc.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, bg)
	}
}

func centreOn(g *globeView, c [3]float32) {
	g.cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, 0)
}

func blend(img *image.RGBA, x, y int, c color.NRGBA, k float64) {
	if !image.Pt(x, y).In(img.Rect) || c.A == 0 {
		return
	}
	a := float64(c.A) / 255 * k
	o := img.RGBAAt(x, y)
	img.SetRGBA(x, y, color.RGBA{
		uint8(float64(o.R)*(1-a) + float64(c.R)*a),
		uint8(float64(o.G)*(1-a) + float64(c.G)*a),
		uint8(float64(o.B)*(1-a) + float64(c.B)*a), 255})
}

func drawOver(img *image.RGBA, src *image.NRGBA, ox, oy int, k float64) {
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			blend(img, x+ox, y+oy, src.NRGBAAt(x, y), k)
		}
	}
}

func disc(img *image.RGBA, cx, cy, r float64, c color.NRGBA) {
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			if math.Hypot(float64(x)-cx, float64(y)-cy) <= r {
				blend(img, x, y, c, 1)
			}
		}
	}
}

// raster fills the triangles with their interpolated vertex colours.
func raster(img *image.RGBA, vs []ebiten.Vertex, idx []uint32) {
	for k := 0; k+2 < len(idx); k += 3 {
		a, b, c := vs[idx[k]], vs[idx[k+1]], vs[idx[k+2]]
		minX := int(math.Floor(float64(min(a.DstX, b.DstX, c.DstX))))
		maxX := int(math.Ceil(float64(max(a.DstX, b.DstX, c.DstX))))
		minY := int(math.Floor(float64(min(a.DstY, b.DstY, c.DstY))))
		maxY := int(math.Ceil(float64(max(a.DstY, b.DstY, c.DstY))))
		den := (b.DstY-c.DstY)*(a.DstX-c.DstX) + (c.DstX-b.DstX)*(a.DstY-c.DstY)
		if den == 0 {
			continue
		}
		for y := max(minY, 0); y <= min(maxY, img.Rect.Max.Y-1); y++ {
			for x := max(minX, 0); x <= min(maxX, img.Rect.Max.X-1); x++ {
				px, py := float32(x)+0.5, float32(y)+0.5
				l1 := ((b.DstY-c.DstY)*(px-c.DstX) + (c.DstX-b.DstX)*(py-c.DstY)) / den
				l2 := ((c.DstY-a.DstY)*(px-c.DstX) + (a.DstX-c.DstX)*(py-c.DstY)) / den
				l3 := 1 - l1 - l2
				if l1 < -0.001 || l2 < -0.001 || l3 < -0.001 {
					continue
				}
				col := func(ca, cb, cc float32) uint8 {
					return uint8(math.Min(255, float64(l1*ca+l2*cb+l3*cc)*255))
				}
				blend(img, x, y, color.NRGBA{col(a.ColorR, b.ColorR, c.ColorR), col(a.ColorG, b.ColorG, c.ColorG),
					col(a.ColorB, b.ColorB, c.ColorB), col(a.ColorA, b.ColorA, c.ColorA)}, 1)
			}
		}
	}
}

var _ = globemath.MinDist
