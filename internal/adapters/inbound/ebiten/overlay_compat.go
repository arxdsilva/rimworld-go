package ebiten

import (
	"image"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The map overlays live in render/overlay (ARCH-004 wave 4.3): stains and
// dirt, fog, roof, home, plan, paths, zones, bridges, turrets, arrivals,
// fire, the power grid and the snow-clearing and allowed areas. These
// aliases and one-line wrappers keep the root's call sites unchanged; the
// last mover deletes this file (SOLID_GO.md §11). The renderer's ov field
// holds the layer, built on first use (overlays).

type (
	areaOverlay        = overlay.AreaOverlay
	bridgeCache        = overlay.BridgeCache
	casketMemo         = overlay.CasketMemo
	eaveEdge           = overlay.EaveEdge
	edgeRun            = overlay.EdgeRun
	fireFx             = overlay.FireFx
	fogCache           = overlay.FogCache
	homeDrawState      = overlay.HomeDrawState
	planDrawState      = overlay.PlanDrawState
	powerDraw          = overlay.PowerDraw
	powerOverlay       = overlay.PowerOverlay
	roofDrawState      = overlay.RoofDrawState
	snowClearDrawState = overlay.SnowClearDrawState
	stainBatch         = overlay.StainBatch
	stainShape         = overlay.StainShape
	zoneGeom           = overlay.ZoneGeom
)

const (
	arrivalDustTicks  = overlay.ArrivalDustTicks
	arrivalFallTicks  = overlay.ArrivalFallTicks
	batteryKind       = overlay.BatteryKind
	beaconBlinkFrames = overlay.BeaconBlinkFrames
	beaconRadiusTiles = overlay.BeaconRadiusTiles
	bloodWetTicks     = overlay.BloodWetTicks
	bridgeKind        = overlay.BridgeKind
	casketKind        = overlay.CasketKind
	casketOpen        = overlay.CasketOpen
	conduitKind       = overlay.ConduitKind
	coolerKind        = overlay.CoolerKind
	dirtPrintN        = overlay.DirtPrintN
	dirtPx            = overlay.DirtPx
	dirtTexN          = overlay.DirtTexN
	geothermalKind    = overlay.GeothermalKind
	heaterKind        = overlay.HeaterKind
	hiddenConduitKind = overlay.HiddenConduitKind
	noPowerIcon       = overlay.NoPowerIcon
	powerSwitchKind   = overlay.PowerSwitchKind
	radioConsoleKind  = overlay.RadioConsoleKind
	roofBuilt         = overlay.RoofBuilt
	roofNatural       = overlay.RoofNatural
	roofPlanBuild     = overlay.RoofPlanBuild
	roofPlanRemove    = overlay.RoofPlanRemove
	roofSupported     = overlay.RoofSupported
	roofThick         = overlay.RoofThick
	sentryTurretKind  = overlay.SentryTurretKind
	sideE             = overlay.SideE
	sideN             = overlay.SideN
	sideS             = overlay.SideS
	sideW             = overlay.SideW
	solarPanelKind    = overlay.SolarPanelKind
	stainDrip         = overlay.StainDrip
	stainPool         = overlay.StainPool
	stainSplat        = overlay.StainSplat
	torchFlameScale   = overlay.TorchFlameScale
	torchLampKind     = overlay.TorchLampKind
	tradeBeaconKind   = overlay.TradeBeaconKind
	turbineFrames     = overlay.TurbineFrames
	turbineStepDeg    = overlay.TurbineStepDeg
	turretAimFrames   = overlay.TurretAimFrames
	turretBaseKey     = overlay.TurretBaseKey
	turretRangeTiles  = overlay.TurretRangeTiles
	turretRecoilTicks = overlay.TurretRecoilTicks
	windTurbineKind   = overlay.WindTurbineKind
	woodGeneratorKind = overlay.WoodGeneratorKind
	zoneLabelMax      = overlay.ZoneLabelMax
	zoneLabelNear     = overlay.ZoneLabelNear
)

var (
	fogClr           = overlay.FogClr
	powerDeviceKinds = overlay.PowerDeviceKinds
	powerPalette     = overlay.PowerPalette
	stockEdgeClr     = overlay.StockEdgeClr
)

func arrivalEffects(pods []game.ArrivalView, now uint64, reduced bool) []overlay.ArrivalFx {
	return overlay.ArrivalEffects(pods, now, reduced)
}

func badgeAlpha(now uint64, reducedFlashing bool) float64 {
	return overlay.BadgeAlpha(now, reducedFlashing)
}

func batteryBarColour(f float64) color.NRGBA { return overlay.BatteryBarColour(f) }

func batteryBarRect(x0, y0, x1, y1, f, th float64) (x, y, w, h float64) {
	return overlay.BatteryBarRect(x0, y0, x1, y1, f, th)
}

func batteryFill(b *game.BuildingView) float64 { return overlay.BatteryFill(b) }

func beaconLit(present uint64, id entity.EntityID, reduced bool) bool {
	return overlay.BeaconLit(present, id, reduced)
}

func bladeKey(frame int) string { return overlay.BladeKey(frame) }

func blazeLight(buf []float32, fires []game.FireView, w, h int) []float32 {
	return overlay.BlazeLight(buf, fires, w, h)
}

func bloodWet(age float64, ok bool) (wetStep int, add float64) { return overlay.BloodWet(age, ok) }

func bridgeCells(w game.WorldView) map[[2]int]bool { return overlay.BridgeCells(w) }

func bridgeKey(m int) string { return overlay.BridgeKey(m) }

func bridgeLabelAt(w game.WorldView, x, y int) string { return overlay.BridgeLabelAt(w, x, y) }

func bridgeLine(b game.BuildingView) string { return overlay.BridgeLine(b) }

func casketSprite(b game.BuildingView) string { return overlay.CasketSprite(b) }

func casketTitle(b game.BuildingView) string { return overlay.CasketTitle(b) }

func coveredVent(covers [][4]int, b *game.BuildingView) bool { return overlay.CoveredVent(covers, b) }

func dashedRun(b *gfx.TriBatch, ax, ay, bx, by, th, dash, gap float32, c color.RGBA, sw, sh int) {
	overlay.DashedRun(b, ax, ay, bx, by, th, dash, gap, c, sw, sh)
}

func decodeBlood(w float32) (coverage float64, wetStep int, ichor bool) {
	return overlay.DecodeBlood(w)
}

func dirtLookOf(d game.DirtView) overlay.DirtLook { return overlay.DirtLookOf(d) }

func dirtPixels(tex int) *image.NRGBA { return overlay.DirtPixels(tex) }

func dirtPrintCount(amount float64) int { return overlay.DirtPrintCount(amount) }

func dirtTexIndex(n, variant int) int { return overlay.DirtTexIndex(n, variant) }

func eaveAlpha(l light.State) uint8 { return overlay.EaveAlpha(l) }

func eaveEdges(w game.WorldView, x0, y0, x1, y1 int, out []overlay.EaveEdge) []overlay.EaveEdge {
	return overlay.EaveEdges(w, x0, y0, x1, y1, out)
}

func eaveWidth(side int, tile float64) float64 { return overlay.EaveWidth(side, tile) }

func encodeBlood(coverage float64, wetStep int, ichor bool) float32 {
	return overlay.EncodeBlood(coverage, wetStep, ichor)
}

func ghostLink(cells [][2]int, w, h int, is func(x, y int) bool) (from, to [2]int, ok bool) {
	return overlay.GhostLink(cells, w, h, is)
}

func glowKey(key string) string { return overlay.GlowKey(key) }

func isConduitKind(kind string) bool { return overlay.IsConduitKind(kind) }

func isHiveSpecies(key string) bool { return overlay.IsHiveSpecies(key) }

func layerBlood(key string, blood float32) float32 { return overlay.LayerBlood(key, blood) }

func noPowerBadge(b *game.BuildingView) bool { return overlay.NoPowerBadge(b) }

func ownsFlame(kind string) bool { return overlay.OwnsFlame(kind) }

func pathStroke(zoom float64) float32 { return overlay.PathStroke(zoom) }

func placementRing(kind string) (float64, bool) { return overlay.PlacementRing(kind) }

func planPaths(pawns []game.PawnView, selected uint64, hasSelected bool, zoom float64, sw, sh int,
	toScreen func(x, y float64) (float32, float32), pos func(game.PawnView) (float64, float64)) []overlay.PawnPathPlan {
	return overlay.PlanPaths(pawns, selected, hasSelected, zoom, sw, sh, toScreen, pos)
}

func recoilPush(shotAge int) float64 { return overlay.RecoilPush(shotAge) }

func roofCellAt(w game.WorldView, x, y int) uint8 { return overlay.RoofCellAt(w, x, y) }

func roofLabelAt(w game.WorldView, x, y int) string { return overlay.RoofLabelAt(w, x, y) }

func roofed(c uint8) bool { return overlay.Roofed(c) }

func roomIDAt(w game.WorldView, x, y int) int { return overlay.RoomIDAt(w, x, y) }

func stainLookOf(s game.StainView) overlay.StainLook { return overlay.StainLookOf(s) }

func stainPixels(shape overlay.StainShape, variant int) *image.NRGBA {
	return overlay.StainPixels(shape, variant)
}

func tokenNRGBA(c color.RGBA) color.NRGBA { return overlay.TokenNRGBA(c) }

// torchFireAt is where torch b's flame stands (its sprite's fire attachment).
func torchFireAt(c *artCatalog, b *game.BuildingView) (float64, float64, bool) {
	return overlay.TorchFireAt(c, buildSprite(torchLampKind, "", 0, false, false), b)
}

func turretAimVec(i int) vec { p := overlay.TurretAimVec(i); return vec{X: p.X, Y: p.Y} }

func turretGunKey(i int) string { return overlay.TurretGunKey(i) }

func turretLine(b game.BuildingView) string { return overlay.TurretLine(b) }

func turretMuzzle(c *artCatalog, b *game.BuildingView) (vec, bool) {
	p, ok := overlay.TurretMuzzle(c, b)
	return vec{X: p.X, Y: p.Y}, ok
}

func unexploredLines(w game.WorldView, x, y int) []string { return overlay.UnexploredLines(w, x, y) }

func ventTraceKey(f, x, y int) string { return overlay.VentTraceKey(f, x, y) }

func worldFogged(w game.WorldView, x, y int) bool { return overlay.WorldFogged(w, x, y) }

func zoneEdgeRuns(cells map[[2]int]bool, order [][2]int) []overlay.EdgeRun {
	return overlay.ZoneEdgeRuns(cells, order)
}

func zoneLabelCap(tileL float64) float64 { return overlay.ZoneLabelCap(tileL) }

func zoneLabelLayout(text string, boxW, boxH, cx, cy, capSize float64) overlay.ZoneLabelPlan {
	return overlay.ZoneLabelLayout(text, boxW, boxH, cx, cy, capSize)
}

func zoneLabelPad(size float64) (px, py float64) { return overlay.ZoneLabelPad(size) }

func zoneLabelText(z game.ZoneView) string { return overlay.ZoneLabelText(z) }

func zoneStroke(zoom float64) float32 { return overlay.ZoneStroke(zoom) }

// overlays is the renderer's map overlays, built on first use with the
// renderer's other layers wired in (overlay.Host), and its painter's
// catalog kept current (the App sets r.art after New).
func (r *Renderer) overlays() *overlay.Set {
	if r.ov == nil {
		p := r.painter()
		r.ov = overlay.New(&p, r.litLayer())
		r.ov.Host = overlay.Host{
			Cue:      r.cue,
			Burst:    r.overlayBurst,
			Flame:    drawFlameScreen,
			Puff:     r.drawPuff,
			PuffAt:   puffAt,
			BuildKey: buildSprite,
			HitAge:   r.hitAge,
			Flash:    r.flashStrike,
		}
	}
	r.ov.P.Art = r.art
	return r.ov
}

// overlayBurst emits an overlay's particles (overlay.Host.Burst).
func (r *Renderer) overlayBurst(p overlay.Particle, x, y float64, n int, seed uint64) {
	kind := partEmber
	if p == overlay.Smoke {
		kind = partSmoke
	}
	r.burst(kind, x, y, n, seed)
}

// flashStrike shows a sim lightning strike of tick: the flash now and the
// thunder rolling in after its delay (overlay.Host.Flash).
func (r *Renderer) flashStrike(tick uint64) {
	e := r.envLayer()
	e.FlashBucket, e.FlashAt, e.FlashSeen = tick/lightningBucket, r.present, true
	r.cueBus().ThunderAt = r.present + thunderDelay(tick)
}

// overlayView is the frame view the overlays read: the sprite view (the
// renderer's live camera, which a bake moves) with the map size, tick,
// selection and options copied in, since tests drive these entry points
// without Renderer.Draw filling the view. The caller puts frame back.
func (r *Renderer) overlayView() (v *render.View, frame camera) {
	v, frame = r.spriteView()
	if r.game != nil {
		v.MapW, v.MapH, v.Tick = r.game.Width(), r.game.Height(), r.game.Tick()
	}
	v.Selected, v.HasSelect = r.selected, r.hasSelect
	v.ReducedMotion, v.ReducedFlashing = r.reducedMotion, r.reducedFlashing
	return v, frame
}

// partView is overlayView for the per-building and per-run entry points
// the world pass calls many times a frame: the sprite view and the
// options, without the simulation's map size and tick.
func (r *Renderer) partView() (v *render.View, frame camera) {
	v, frame = r.spriteView()
	v.ReducedMotion, v.ReducedFlashing = r.reducedMotion, r.reducedFlashing
	return v, frame
}

// overlayChunks is the frame's chunk lists and view as the overlays walk
// them (lod.go).
func (r *Renderer) overlayChunks() overlay.Chunks {
	l := &r.lod
	return overlay.Chunks{Views: l.Views, VX0: l.VX0, VY0: l.VY0, VX1: l.VX1, VY1: l.VY1, ViewSet: l.ViewSet,
		Live: l.Tier == lodLive, WholeMap: l.Tier == lodMap, Baking: l.Baking}
}

// areasOf is the simulation's allowed areas (asked for only while one is
// armed or hovered).
func (r *Renderer) areasOf() []game.AreaView { return r.game.Areas() }

// ---- the toggles

func (r *Renderer) roofOverlayOn() bool { return r.overlays().Roof.Shown() }
func (r *Renderer) homeOverlayOn() bool { return r.overlays().Home.Shown() }
func (r *Renderer) planOverlayOn() bool { return r.overlays().Plan.Shown() }
func (r *Renderer) powerShown() bool    { return r.overlays().Power.Shown() }
func (r *Renderer) togglePowerOverlay() { r.overlays().Power.Toggle() }

// togglePowerOverlayIfAny tolerates a nil renderer (menus).
func (r *Renderer) togglePowerOverlayIfAny() {
	if r != nil {
		r.togglePowerOverlay()
	}
}

// ---- the layers

func (r *Renderer) drawZones(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Zones.DrawZones(v, r.spritePass(), &ch, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneLabels(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Zones.DrawZoneLabels(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneLabel(screen *ebiten.Image, z game.ZoneView, g *zoneGeom, sw, sh int, tile float64) {
	v, frame := r.partView()
	r.overlays().Zones.DrawZoneLabel(v, screen, z, g, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneRun(b *triBatch, kind string, run edgeRun, sw, sh int, tile float64) {
	v, frame := r.partView()
	r.overlays().Zones.DrawZoneRun(v, b, kind, run, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawFenceRun(b *triBatch, run edgeRun, tile float64) {
	v, frame := r.partView()
	r.overlays().Zones.DrawFenceRun(v, b, run, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneHighlight(screen *ebiten.Image, w game.WorldView, z game.ZoneView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Zones.DrawZoneHighlight(v, &ch, screen, w, z, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) zoneGeomOf(z game.ZoneView) *zoneGeom { return r.overlays().Zones.ZoneGeomOf(z) }

func (r *Renderer) zoneLabelPlanFor(z game.ZoneView, g *zoneGeom, tile float64) (plan overlay.ZoneLabelPlan, ox, oy float32, bw, bh float64) {
	v, frame := r.partView()
	defer func() { v.Cam = frame }()
	return r.overlays().Zones.ZoneLabelPlanFor(v, z, g, tile)
}

func (r *Renderer) zoneLabelAt(w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool) {
	v, frame := r.overlayView()
	defer func() { v.Cam = frame }()
	return r.overlays().Zones.ZoneLabelAt(v, w, mx, my, tile, shown)
}

func (r *Renderer) drawEaves(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawEaves(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawRoofOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawRoofOverlay(v, screen, w, r.areasOf, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawRoofUnsupported(screen *ebiten.Image, x, y int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawRoofUnsupported(v, screen, x, y, tile)
	v.Cam = frame
}

func (r *Renderer) drawHomeOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawHomeOverlay(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawPlanOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawPlanOverlay(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawSnowClearOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawSnowClearOverlay(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawAreaOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawAreaOverlay(v, screen, w, r.areasOf, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawAreas(screen *ebiten.Image, w game.WorldView, areas []game.AreaView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawAreas(v, screen, w, areas, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawPaths(screen *ebiten.Image, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawPaths(v, screen, sw, sh, tile, r.interpWorld)
	v.Cam = frame
}

func (r *Renderer) drawFog(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawFog(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawFogLayer(screen *ebiten.Image, fog []uint8, ver uint64, mw, mh int, tile float64, sw, sh int) {
	v, frame := r.overlayView()
	r.overlays().DrawFogLayer(v, screen, fog, ver, mw, mh, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawStains(v, r.spritePass(), screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) buildStains(b *stainBatch, w game.WorldView, sw, sh int, tile float64) int {
	v, frame := r.overlayView()
	defer func() { v.Cam = frame }()
	return r.overlays().BuildStains(v, b, w, sw, sh, tile)
}

func (r *Renderer) drawDirt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawDirt(v, r.spritePass(), screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) buildDirt(b *stainBatch, w game.WorldView, sw, sh int, tile float64) int {
	v, frame := r.overlayView()
	defer func() { v.Cam = frame }()
	return r.overlays().BuildDirt(v, b, w, sw, sh, tile)
}

// pawnBlood is the Custom3 blood value for p's figure this frame.
func (r *Renderer) pawnBlood(p game.PawnView) float32 {
	v := r.lightView()
	if p.HitTick != 0 {
		v.AnimT = r.animT()
	}
	return r.overlays().PawnBlood(v, r.spritePass(), p)
}

// animalBlood is the Custom3 blood value for an animal's figure this frame.
func (r *Renderer) animalBlood(an game.AnimalView) float32 {
	v := r.lightView()
	if an.HitTick != 0 {
		v.AnimT = r.animT()
	}
	return r.overlays().AnimalBlood(v, r.spritePass(), an)
}

func (r *Renderer) drawFoundation(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().DrawFoundation(v, r.spritePass(), &ch, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawBridgeCell(dst *ebiten.Image, w game.WorldView, set map[[2]int]bool, b *game.BuildingView, tile float64, sw, sh int) {
	v, frame := r.partView()
	r.overlays().DrawBridgeCell(v, r.spritePass(), dst, w, set, b, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) buildingByID(id entity.EntityID) (*game.BuildingView, bool) {
	return r.overlays().BuildingByID(&r.view, id)
}

func (r *Renderer) drawDefenseParts(dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	v, frame := r.partView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawDefenseParts(v, r.spritePass(), &ch, dst, b, key, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawRangeRing(dst *ebiten.Image, x, y int, radius float64, c color.NRGBA) {
	v, frame := r.partView()
	r.overlays().Wiring.DrawRangeRing(v, dst, x, y, radius, c)
	v.Cam = frame
}

func (r *Renderer) drawArrivals(screen *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawArrivals(v, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) drawFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Fires.DrawFires(v, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) emitFireParticles(w game.WorldView) {
	v, frame := r.overlayView()
	r.overlays().Fires.EmitFireParticles(v, w)
	v.Cam = frame
}

func (r *Renderer) drawWorldFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Fires.DrawWorldFires(v, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) emitBlazeParticles(w game.WorldView) {
	v, frame := r.overlayView()
	r.overlays().Fires.EmitBlazeParticles(v, w)
	v.Cam = frame
}

func (r *Renderer) drawAshStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Fires.DrawAshStains(v, r.spritePass(), screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) onSimStrike(x, y int, tick uint64) { r.overlays().Fires.OnSimStrike(x, y, tick) }

func (r *Renderer) showStrikes(w game.WorldView) {
	v, frame := r.overlayView()
	r.overlays().Fires.ShowStrikes(v, w)
	v.Cam = frame
}

// blazeAmbience adds one positioned amb.fire loop per fire cluster, the
// loudest few (overlay.Set.BlazeAmbience).
func (r *Renderer) blazeAmbience(want map[string]loopWant, w game.WorldView, vp viewport) {
	r.overlays().Fires.BlazeAmbience(w,
		func(x, y float64) (float64, float64, bool) { return spatial(r, x, y, vp) },
		func(key string, vol, pan float64) { want[key] = loopWant{ID: "amb.fire", Vol: vol, Pan: pan} })
}

func (r *Renderer) listenCaskets(w game.WorldView) { r.overlays().ListenCaskets(w) }

func (r *Renderer) powerTrace(part, key string) { r.overlays().Wiring.PowerTrace(part, key) }

func (r *Renderer) drawPowerParts(dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	v, frame := r.partView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawPowerParts(v, r.spritePass(), &ch, dst, b, key, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawConduits(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawConduits(v, r.spritePass(), &ch, &r.links, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawPowerMarkings(dst *ebiten.Image, w *game.WorldView, tile float64, sw, sh int, colour func(net int) color.NRGBA, hiddenOnly bool) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawPowerMarkings(v, &ch, &r.links, dst, w, tile, sw, sh, colour, hiddenOnly)
	v.Cam = frame
}

func (r *Renderer) drawPowerOverlay(screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().DrawPowerOverlay(v, &ch, &r.links, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawNoPowerBadges(screen *ebiten.Image, w *game.WorldView, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Wiring.DrawNoPowerBadges(v, &ch, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) drawMachinePuff(screen *ebiten.Image, now uint64, b *game.BuildingView, tile float64) {
	v, frame := r.partView()
	r.overlays().Wiring.DrawMachinePuff(v, screen, now, b, tile)
	v.Cam = frame
}

func (r *Renderer) drawPowerLink(dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64) {
	v, frame := r.partView()
	r.overlays().Wiring.DrawPowerLink(v, dst, from, to, c, tile)
	v.Cam = frame
}

func casketSealed(b game.BuildingView) bool { return overlay.CasketSealed(b) }

func (r *Renderer) bladeAt(b *game.BuildingView, baseKey, bladeK string) (float64, float64, bool) {
	v := r.lightView()
	return r.overlays().Wiring.BladeAt(v, b, baseKey, bladeK)
}

// ---- kept in the root: a hit's blood spray is a particle (render/creature)

// sprayFor is a hit's spray: 4 + damage/2 drops (at most 16), 1.25×
// faster for hits of 10 damage or more.
func sprayFor(dmg int) (n int, speed float64) {
	n = min(4+max(dmg, 0)/2, 16)
	speed = bloodSpeed
	if dmg >= 10 {
		speed *= 1.25
	}
	return n, speed
}

// sprayKind is what a hit target bleeds: ichor for hive insects.
func sprayKind(w game.WorldView, target entity.EntityID) partKind {
	if target != 0 {
		for i := range w.Animals {
			if an := &w.Animals[i]; an.ID == target {
				if isHiveSpecies(an.Species) {
					return partIchor
				}
				break
			}
		}
	}
	return partBlood
}
