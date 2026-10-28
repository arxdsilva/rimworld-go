package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// The drawing primitives live in gfx (ARCH-003). These aliases and
// one-line wrappers keep the root's call sites unchanged while its
// features move out one package at a time; each mover repoints its own
// files to gfx and the last one deletes this file (SOLID_GO.md §11: no
// forwarder outlives the commit series that repoints its callers).

type (
	rect            = gfx.Rect
	textStyle       = gfx.TextStyle
	align           = gfx.Align
	triBatch        = gfx.TriBatch
	uiScaleState    = gfx.Scale
	traceEvent      = gfx.TraceEvent
	worldDeferQueue = gfx.WorldPass
)

const (
	alignLeft   = gfx.AlignLeft
	alignCenter = gfx.AlignCenter
	alignRight  = gfx.AlignRight
)

var (
	tsSmall     = gfx.Small
	tsBody      = gfx.Body
	tsLabel     = gfx.Label
	tsLarge     = gfx.Large
	tsTitle     = gfx.Title
	tsHuge      = gfx.Huge
	unitUIScale = gfx.UnitScale
)

func uiFillRect(dst *ebiten.Image, x, y, w, h float32, clr color.Color, aa bool) {
	gfx.FillRect(dst, x, y, w, h, clr, aa)
}

func uiStrokeRect(dst *ebiten.Image, x, y, w, h, width float32, clr color.Color, aa bool) {
	gfx.StrokeRect(dst, x, y, w, h, width, clr, aa)
}

func uiLine(dst *ebiten.Image, x0, y0, x1, y1, width float32, clr color.Color, aa bool) {
	gfx.Line(dst, x0, y0, x1, y1, width, clr, aa)
}

func uiFillCircle(dst *ebiten.Image, cx, cy, r float32, clr color.Color, aa bool) {
	gfx.FillCircle(dst, cx, cy, r, clr, aa)
}

func uiStrokeCircle(dst *ebiten.Image, cx, cy, r, width float32, clr color.Color, aa bool) {
	gfx.StrokeCircle(dst, cx, cy, r, width, clr, aa)
}

func uiFillPoly(dst *ebiten.Image, pts [][2]float32, clr color.Color, aa bool) {
	gfx.FillPoly(dst, pts, clr, aa)
}

func uiImageOp(op *ebiten.DrawImageOptions) { gfx.ImageOp(op) }

func uiDrawImage(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) { gfx.DrawImage(dst, img, op) }

func uiImage(dst, img *ebiten.Image, x, y, w, h float64, opts *ebiten.DrawImageOptions) {
	gfx.Image(dst, img, x, y, w, h, opts)
}

func uiTriangles(dst *ebiten.Image, vs []ebiten.Vertex, is []uint16, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	gfx.Triangles(dst, vs, is, img, opts)
}

func uiTriangles32(dst *ebiten.Image, vs []ebiten.Vertex, is []uint32, img *ebiten.Image, opts *ebiten.DrawTrianglesOptions) {
	gfx.Triangles32(dst, vs, is, img, opts)
}

func uiPhysRect(r rect) rect { return gfx.PhysRect(r) }

func worldText(dst *ebiten.Image, s string, physX, physY float64, st textStyle) {
	gfx.WorldText(dst, s, physX, physY, st)
}

func worldIcon(dst, img *ebiten.Image, physX, physY, size float64) {
	gfx.WorldIcon(dst, img, physX, physY, size)
}

func drawIcon(dst, img *ebiten.Image, x, y, size float64) { gfx.Icon(dst, img, x, y, size) }

func drawText(dst *ebiten.Image, s string, x, y float64, st textStyle) { gfx.Text(dst, s, x, y, st) }

func measure(s string, st textStyle) (w, h float64) { return gfx.Measure(s, st) }

func lineH(st textStyle) float64 { return gfx.LineH(st) }

func wrapPx(s string, st textStyle, maxW float64) []string { return gfx.WrapPx(s, st, maxW) }

func ellipsize(s string, st textStyle, maxW float64) string { return gfx.Ellipsize(s, st, maxW) }

func withColor(st textStyle, c color.Color) textStyle { return gfx.WithColor(st, c) }

func lineStep(st textStyle) float32 { return gfx.LineStep(st) }

func maxTextW(lines []string, st textStyle) float32 { return gfx.MaxTextW(lines, st) }

func winRect(sw, sh int) rect { return gfx.WinRect(sw, sh) }

func clipTo(dst *ebiten.Image, r rect) *ebiten.Image { return gfx.ClipTo(dst, r) }

func fitLayer(img *ebiten.Image, sw, sh int) *ebiten.Image { return gfx.FitLayer(img, sw, sh) }

func compositeWorld(screen, world *ebiten.Image, k float64) { gfx.CompositeWorld(screen, world, k) }

func uiScale() float64 { return gfx.UIScale() }

func hudScale() float64 { return gfx.HUDScale() }

func mapUIScale() float64 { return gfx.MapUIScale() }

func clamp01(t float64) float64 { return gfx.Clamp01(t) }

func setTracePass(p string) { gfx.SetPass(p) }

func textW(s string, st textStyle) float32 { return gfx.TextW(s, st) }

func abs32(v float32) float32 { return gfx.Abs32(v) }
