package ebiten

import (
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Electricity on the map (PLAY-021, RENDER). Presentation only: every
// part reads the BuildingView power fields (PowerRole, Powered, PowerOn,
// PowerOut, StoredWh/CapWh, NoPowerReason, Glow) and the world's wind,
// and nothing here changes state.
//
//   - Conduits have their own pass (drawConduits) over the floors and
//     under zones and objects, linked on the power layer of linkgrid.go;
//     hidden conduits draw nothing there, only in the power overlay
//     (drawPowerMarkings, called by POWERUI's power_overlay.go).
//   - A device's glow layer (<sprite>_glow[_<facing>], same canvas and
//     origin) is drawn unlit over its sprite while it runs (poweredLook).
//   - The wind turbine's blades hang their hub on the base's rotor and
//     turn with the wind on the present clock; the torch lamp burns with
//     drawFlame at its fire attachment; a battery shows its charge between
//     its gauge attachments; generators smoke and steam (features_draw.go).
//   - Unpowered consumers wear a softly blinking no-power badge.
//
// None of it is drawn while baking: bakes are cached on the chunk
// version, and power changes without a rebake.

// The PLAY-021 building kinds the renderer draws (construction.go names).
const (
	conduitKind       = "power conduit"
	hiddenConduitKind = "hidden conduit"
	powerSwitchKind   = "power switch"
	woodGeneratorKind = "wood-fired generator"
	solarPanelKind    = "solar panel"
	windTurbineKind   = "wind turbine"
	geothermalKind    = "geothermal generator"
	batteryKind       = "battery"
	torchLampKind     = "torch lamp"
	heaterKind        = "heater"
	coolerKind        = "cooler"
)

// isConduitKind reports a conduit, shown or hidden (the conduit pass).
func isConduitKind(kind string) bool { return kind == conduitKind || kind == hiddenConduitKind }

// powerDeviceKinds are the kinds that make, draw or store power: a
// conduit's stub reaches into them. Built devices also count by their
// view's PowerRole; frames and ghosts, which have none, by this list.
var powerDeviceKinds = map[string]bool{
	"lamp": true, heaterKind: true, coolerKind: true, batteryKind: true,
	woodGeneratorKind: true, solarPanelKind: true, windTurbineKind: true, geothermalKind: true,
	"electric stove": true, "electric tailoring bench": true, "electric research bench": true,
}

// powerDraw is the renderer's power presentation state (Renderer.power):
// each turbine's blade phase, the glow key memo and the reused vent
// cover list. trace, when set (tests), sees every part drawn.
type powerDraw struct {
	spins  map[entity.EntityID]turbineSpin
	pruned uint64
	glow   map[string]string
	covers [][4]int
	trace  func(part, key string)
}

// powerTrace reports a part to the test hook.
func (r *Renderer) powerTrace(part, key string) {
	if r.power.trace != nil {
		r.power.trace(part, key)
	}
}

// glowKey is the glow layer of a sprite key: <name>_glow, or for a faced
// sprite <name>_glow_<facing> (the art's naming; TestPowerArtCoversRenderer
// checks it against every base's metadata "glow:").
func glowKey(key string) string {
	for _, f := range facingNames {
		if base, ok := strings.CutSuffix(key, "_"+f); ok {
			return base + "_glow_" + f
		}
	}
	return key + "_glow"
}

// glowOf is glowKey memoised (one string per sprite, not per frame).
func (p *powerDraw) glowOf(key string) string {
	if g, ok := p.glow[key]; ok {
		return g
	}
	if p.glow == nil {
		p.glow = map[string]string{}
	}
	g := glowKey(key)
	p.glow[key] = g
	return g
}

// poweredLook reports a device that is running now, so its glow layer
// lights: a consumer while Powered, a producer while it makes power, a
// battery while it holds charge on a net. Never while unpowered.
func poweredLook(b *game.BuildingView) bool {
	switch b.PowerRole {
	case "consumer":
		return b.Powered
	case "producer":
		return b.PowerOut > 0
	case "storage":
		return b.PowerNet != 0 && b.StoredWh > 0
	}
	return false
}

// ownsFlame reports a burning kind whose fire power_draw.go draws (the
// torch's flame) or that shows none (the generator's firebox glows and
// smokes): drawFires and the campfire embers leave them alone.
func ownsFlame(kind string) bool { return kind == torchLampKind || kind == woodGeneratorKind }

// attachWorld is attachment name of sprite key drawn for a building
// anchored at (x, y), in world tiles: placement puts the sprite's
// origin_px on the anchor (artOrigin). ok is false without it. It reads
// the metadata only, never the pixels.
func attachWorld(c *artCatalog, key, name string, x, y float64) (float64, float64, bool) {
	if c == nil {
		return 0, 0, false
	}
	sp := c.sprites[key]
	if sp == nil {
		return 0, 0, false
	}
	a, ok := sp.meta.Attach[name]
	if !ok {
		return 0, 0, false
	}
	return x + (a[0]-sp.meta.Origin[0])/artTilePx, y + (a[1]-sp.meta.Origin[1])/artTilePx, true
}

// drawUnlitSprite draws key unlit at world (wx, wy), multiplied by cs:
// glow layers and overlay markings, which give their own light. false
// when nothing was drawn (no art, off screen).
func (r *Renderer) drawUnlitSprite(dst *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs colorScale) bool {
	sp := r.art.sprite(key)
	if sp == nil {
		return false
	}
	sx, sy := r.worldToScreenF(wx, wy)
	x, y, s := artOrigin(sp.meta, float64(sx), float64(sy), tile)
	if !visible(float32(x), float32(y), float32(float64(sp.meta.Canvas[0])*s), float32(float64(sp.meta.Canvas[1])*s), sw, sh) {
		return false
	}
	img := r.art.image(key, color.NRGBA{})
	if img == nil {
		return false
	}
	var geo ebiten.GeoM
	geo.Scale(s, s)
	geo.Translate(x, y)
	r.drawUnlit(dst, img, geo, cs)
	return true
}

// drawPowerParts draws what a building's power state adds over its
// sprite (key, just drawn): the glow layer while it runs, then the
// turbine's blades, the torch's flame or the battery's charge bar.
func (r *Renderer) drawPowerParts(dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	if r.lod.baking || r.art == nil || (b.PowerRole == "" && b.Kind != torchLampKind) {
		return
	}
	if poweredLook(b) {
		if g := r.power.glowOf(key); r.art.has(g) && r.drawUnlitSprite(dst, g, float64(b.X), float64(b.Y), tile, sw, sh, noCS) {
			r.powerTrace("glow", g)
		}
	}
	switch b.Kind {
	case torchLampKind:
		if torchBurning(b) {
			r.drawTorchFlame(dst, b, tile)
		}
	case windTurbineKind:
		r.drawTurbineBlades(dst, b, key, tile, sw, sh)
	case batteryKind:
		r.drawBatteryBar(dst, b, key, tile)
	}
}

// --- conduits ----------------------------------------------------------

// drawConduits is the conduit pass: over the floors, under zones and
// objects, every shown conduit of the chunks in view, linked to the
// transmitters and devices beside it (linkGrid.conduitMask), then the
// conduit blueprints (linked with the planned set too) and their
// deconstruct marks. A hidden conduit draws only its blueprint and mark
// here; built, it shows in the power overlay alone. Not on the
// whole-map bake (its tiles are a few pixels wide).
func (r *Renderer) drawConduits(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if r.lod.tier == lodMap || r.art == nil {
		return
	}
	none := color.NRGBA{}
	traced, planned := false, false
	var p map[[2]int]linkFlag
	for _, cv := range r.lod.views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !isConduitKind(b.Kind) || !r.lod.inView(b.X, b.Y, 1, 1) {
				continue
			}
			if !traced {
				traced = true
				r.layer("conduits")
			}
			if b.Kind == conduitKind {
				if key := buildSprite(b.Kind, "", r.links.conduitMask(b.X, b.Y), false, false); r.art.has(key) {
					r.powerTrace("conduit", key)
					r.drawSpriteCS(screen, key, none, float64(b.X), float64(b.Y), tile, sw, sh, noCS)
				}
			}
			if b.Deconstruct {
				r.drawSprite(screen, "overlay/designation_deconstruct", none, float64(b.X), float64(b.Y), tile, sw, sh)
			}
		}
		for _, i := range cv.Frames {
			if int(i) >= len(w.Frames) {
				continue
			}
			f := &w.Frames[i]
			if !isConduitKind(f.Kind) || !r.lod.inView(f.X, f.Y, 1, 1) {
				continue
			}
			if !planned { // the frames' planned set, once (O(frames))
				planned = true
				p = r.links.plan(w.Frames, nil)
			}
			key := buildSprite(f.Kind, "", r.links.powerMask(conduitLink.accepts, f.X, f.Y, p), false, false)
			if r.art.has(key) {
				r.powerTrace("conduit frame", key)
				r.drawSpriteCS(screen, key, none, float64(f.X), float64(f.Y), tile, sw, sh, blueprintCS)
			}
		}
	}
}

// drawPowerMarkings draws the power overlay's conduit markings (POWERUI's
// power_overlay.go calls it after the world pass): the hidden-conduit
// marking of every conduit in view (only the hidden ones when
// hiddenOnly), linked like the conduits, multiplied by colour(its net)
// and unlit, so a net reads as one colour by night too.
func (r *Renderer) drawPowerMarkings(dst *ebiten.Image, w *game.WorldView, tile float64, sw, sh int, colour func(net int) color.NRGBA, hiddenOnly bool) {
	if r.art == nil || colour == nil {
		return
	}
	r.links.sync(*w, r.game.Width(), r.game.Height()) // a no-op on the frame's StructVer
	for _, cv := range r.lod.views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !isConduitKind(b.Kind) || (hiddenOnly && b.Kind != hiddenConduitKind) || !r.lod.inView(b.X, b.Y, 1, 1) {
				continue
			}
			key := buildSprite(hiddenConduitKind, "", r.links.conduitMask(b.X, b.Y), false, false)
			c := colour(b.PowerNet)
			cs := colorScale{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
			if r.drawUnlitSprite(dst, key, float64(b.X), float64(b.Y), tile, sw, sh, cs) {
				r.powerTrace("marking", key)
			}
		}
	}
}

// --- wind turbine -----------------------------------------------------

// The blade frames (metadata frames and step_deg): eight frames 15°
// apart loop a three-bladed rotor through a third of a turn.
const (
	turbineFrames  = 8
	turbineStepDeg = 15
	// turbineFPSPerWind is the frames per second per tile/s of wind (a
	// light breeze of 0.5 turns a revolution in 6 s, a storm of 3.5 in
	// 0.9 s); turbineMaxFPS caps it below strobing; under turbineCalm the
	// blades stand still.
	turbineFPSPerWind = 8.0
	turbineMaxFPS     = 30.0
	turbineCalm       = 0.05
	// a turbine unseen this long (present frames) loses its phase
	turbineForget = 600
)

// turbineSpin is one turbine's blade phase (frames, 0..turbineFrames)
// and the present frame it was last advanced to.
type turbineSpin struct {
	phase float64
	last  uint64
}

// turbineFPS is how fast the blades of turbine b turn in the wind
// (windX, windY tiles per second): 0 while it is switched off (PowerOn is
// the actual state) or the air is calm.
func turbineFPS(windX, windY float64, b *game.BuildingView) float64 {
	s := math.Hypot(windX, windY)
	if !b.PowerOn || s < turbineCalm {
		return 0
	}
	return math.Min(s*turbineFPSPerWind, turbineMaxFPS)
}

// bladeFrame advances turbine id's phase to present frame now at fps
// frames per second and returns its blade frame (1..turbineFrames). The
// phase accumulates, so a change of wind changes the speed, never the
// angle; a new turbine starts at a phase of its own.
func (p *powerDraw) bladeFrame(id entity.EntityID, fps float64, now uint64) int {
	if p.spins == nil {
		p.spins = map[entity.EntityID]turbineSpin{}
	}
	s, ok := p.spins[id]
	switch {
	case !ok:
		s = turbineSpin{phase: float64(mix64(uint64(id)) % turbineFrames), last: now}
	case now > s.last:
		s.phase = math.Mod(s.phase+float64(now-s.last)*fps/60, turbineFrames)
		s.last = now
	}
	p.spins[id] = s
	return int(s.phase)%turbineFrames + 1
}

// prune forgets the turbines not drawn for turbineForget frames (gone or
// long off screen), at most once per turbineForget frames.
func (p *powerDraw) prune(now uint64) {
	if now-p.pruned < turbineForget {
		return
	}
	p.pruned = now
	for id, s := range p.spins {
		if now-s.last > turbineForget {
			delete(p.spins, id)
		}
	}
}

// bladeKey is the blade frame's sprite, power/wind_turbine_blades_NN.
func bladeKey(frame int) string { return "power/wind_turbine_blades_" + twoDigits(frame) }

// bladeAt is where the blade sprite bladeK is anchored so its hub lands
// on the rotor of turbine b drawn as baseKey.
func (r *Renderer) bladeAt(b *game.BuildingView, baseKey, bladeK string) (float64, float64, bool) {
	rx, ry, ok := attachWorld(r.art, baseKey, "rotor", float64(b.X), float64(b.Y))
	if !ok {
		return 0, 0, false
	}
	bl := r.art.sprites[bladeK]
	if bl == nil {
		return 0, 0, false
	}
	hub, ok := bl.meta.Attach["hub"]
	if !ok {
		return 0, 0, false
	}
	return rx - (hub[0]-bl.meta.Origin[0])/artTilePx, ry - (hub[1]-bl.meta.Origin[1])/artTilePx, true
}

// drawTurbineBlades draws turbine b's blades over its base (key), on the
// frame its phase has reached; Reduced Motion holds them still.
func (r *Renderer) drawTurbineBlades(dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	fps := turbineFPS(r.view.W.WindX, r.view.W.WindY, b)
	if r.reducedMotion {
		fps = 0
	}
	k := bladeKey(r.power.bladeFrame(b.ID, fps, r.present))
	wx, wy, ok := r.bladeAt(b, key, k)
	if !ok {
		return
	}
	r.powerTrace("blades", k)
	r.drawSpriteCS(dst, k, color.NRGBA{}, wx, wy, tile, sw, sh, noCS)
}

// --- torch lamp --------------------------------------------------------

// torchFlameScale is the torch's flame size against the campfire's
// (metadata flame_scale; TestPowerArtCoversRenderer keeps them equal).
const torchFlameScale = 0.55

// torchBurning reports a torch lamp on fire: the sim says Burning, or it
// gives light (Glow). One with fuel the rain put out is dark.
func torchBurning(b *game.BuildingView) bool { return b.Burning || b.Glow }

// torchFireAt is where torch b's flame stands: its sprite's fire
// attachment (ok false without one).
func torchFireAt(c *artCatalog, b *game.BuildingView) (float64, float64, bool) {
	return attachWorld(c, buildSprite(torchLampKind, "", 0, false, false), "fire", float64(b.X), float64(b.Y))
}

// drawTorchFlame draws the campfire flame, scaled down, on the torch's
// head (its tile's top centre without the attachment).
func (r *Renderer) drawTorchFlame(dst *ebiten.Image, b *game.BuildingView, tile float64) {
	x, y, ok := torchFireAt(r.art, b)
	if !ok {
		x, y = float64(b.X)+0.5, float64(b.Y)
	}
	r.powerTrace("flame", buildSprite(torchLampKind, "", 0, false, false))
	r.drawFlame(dst, x, y, float64(r.present)+float64(b.ID)*17, tile, torchFlameScale)
}

// --- battery -----------------------------------------------------------

// batteryBarPx is the charge bar's thickness in canvas pixels (the gauge
// slot the art leaves between gauge0 and gauge1 is about 12).
const batteryBarPx = 9.0

// batteryFill is battery b's charge, 0..1.
func batteryFill(b *game.BuildingView) float64 {
	if b.CapWh <= 0 || b.StoredWh <= 0 {
		return 0
	}
	return math.Min(float64(b.StoredWh)/float64(b.CapWh), 1)
}

// batteryBarRect is the screen rectangle of a bar th thick that runs
// from (x0, y0), empty, toward (x1, y1), full, filled f of the way. The
// gauges lie along one axis (vertical facing south or north, horizontal
// east or west).
func batteryBarRect(x0, y0, x1, y1, f, th float64) (x, y, w, h float64) {
	ex, ey := x0+(x1-x0)*f, y0+(y1-y0)*f
	if math.Abs(x1-x0) >= math.Abs(y1-y0) {
		return math.Min(x0, ex), y0 - th/2, math.Abs(ex - x0), th
	}
	return x0 - th/2, math.Min(y0, ey), th, math.Abs(ey - y0)
}

// batteryBarColour is green, amber below a quarter.
func batteryBarColour(f float64) color.NRGBA {
	if f < 0.25 {
		return color.NRGBA{R: 226, G: 170, B: 64, A: 235}
	}
	return color.NRGBA{R: 104, G: 200, B: 112, A: 235}
}

// drawBatteryBar draws battery b's charge in its gauge slot, unlit (an
// indicator), from gauge0 toward gauge1.
func (r *Renderer) drawBatteryBar(dst *ebiten.Image, b *game.BuildingView, key string, tile float64) {
	f := batteryFill(b)
	if f <= 0 {
		return
	}
	x0, y0, ok0 := attachWorld(r.art, key, "gauge0", float64(b.X), float64(b.Y))
	x1, y1, ok1 := attachWorld(r.art, key, "gauge1", float64(b.X), float64(b.Y))
	if !ok0 || !ok1 {
		return
	}
	sx0, sy0 := r.worldToScreenF(x0, y0)
	sx1, sy1 := r.worldToScreenF(x1, y1)
	x, y, w, h := batteryBarRect(float64(sx0), float64(sy0), float64(sx1), float64(sy1), f, batteryBarPx*tile/artTilePx)
	r.powerTrace("battery", key)
	r.solidRect(dst, float32(x), float32(y), float32(w), float32(h), batteryBarColour(f))
}

// --- no-power badge -----------------------------------------------------

// noPowerIcon is the badge over an unpowered consumer.
const noPowerIcon = "ui_icon/no_power"

// The badge: badgeTiles of a tile wide, raised badgeLift tiles above its
// footprint's centre, its alpha easing between 1 and 1-badgeDip over
// badgePeriod present frames.
const (
	badgeTiles  = 0.42
	badgeLift   = 0.35
	badgeDip    = 0.55
	badgePeriod = 90
)

// noPowerBadge reports a consumer that wants power and has none (the
// player switching it off is no fault; producers have their own lines).
func noPowerBadge(b *game.BuildingView) bool {
	return b.PowerRole == "consumer" && b.NoPowerReason != "" && b.NoPowerReason != game.NoPowerSwitchedOff
}

// badgeAlpha is the badge's soft blink at present frame now: a cosine
// ease between 1 and 1-badgeDip, steady at 1 under Reduced Flashing.
func badgeAlpha(now uint64, reducedFlashing bool) float64 {
	if reducedFlashing {
		return 1
	}
	t := float64(now%badgePeriod) / badgePeriod
	return 1 - badgeDip*(0.5-0.5*math.Cos(2*math.Pi*t))
}

// drawNoPowerBadges draws the badge over every unpowered consumer in
// view, after the objects so nothing hides it. Not on the whole-map
// bake.
func (r *Renderer) drawNoPowerBadges(screen *ebiten.Image, w *game.WorldView, tile float64) {
	if r.art == nil || r.lod.tier == lodMap {
		return
	}
	var img *ebiten.Image
	a := float32(badgeAlpha(r.present, r.reducedFlashing))
	size := tile * badgeTiles // physical, like the anchor
	for _, cv := range r.lod.views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !noPowerBadge(b) || !r.lod.inView(b.X, b.Y, 1, 1) {
				continue
			}
			if img == nil {
				if img = r.art.image(noPowerIcon, color.NRGBA{}); img == nil {
					return
				}
			}
			sx, sy := r.worldToScreenF(float64(b.X)+float64(max(b.W, 1))/2, float64(b.Y)+float64(max(b.H, 1))/2-badgeLift)
			r.powerTrace("badge", noPowerIcon)
			worldIconCS(screen, img, float64(sx)-size/2, float64(sy)-size/2, size/uiScale(), colorScale{a, a, a, a})
		}
	}
}

// worldIconCS is worldIcon (ui_canvas.go) multiplied by cs: a physical
// anchor, a logical size, converted once.
func worldIconCS(dst, img *ebiten.Image, physX, physY, size float64, cs colorScale) {
	k := 1 / uiScale()
	if gfx.Traced() {
		defer gfx.SetVia(gfx.Via())
		gfx.SetVia("worldIcon")
	}
	drawIconCS(dst, img, physX*k, physY*k, size, cs)
}

// --- generator exhaust (drawn by drawSteamPuffs, features_draw.go) -----

// A running machine's exhaust: four puff frames every period ticks of
// the sim clock, smoke from the wood-fired generator's chimney often,
// steam from the geothermal generator's vent less so.
const (
	smokePeriodMin, smokePeriodSpread = 48, 30
	geoPeriodMin, geoPeriodSpread     = 110, 60
	smokeScale, geoSteamScale         = 0.55, 0.8
)

// smokeCS darkens the steam puff into wood smoke.
var smokeCS = colorScale{0.4, 0.38, 0.36, 0.8}

// machinePuff is the puff frame and alpha of machine id at tick now (ok
// false between puffs), steam or smoke timing, offset by its id.
func machinePuff(now, id uint64, steam bool) (int, float64, bool) {
	h := mix64(id ^ 0x5e4a9)
	lo, spread := uint64(smokePeriodMin), uint64(smokePeriodSpread)
	if steam {
		lo, spread = geoPeriodMin, geoPeriodSpread
	}
	period := lo + h>>8%(spread+1)
	return puffAt((now + h>>24%period) % period)
}

// drawMachinePuff draws running generator b's exhaust at its smoke or
// steam attachment.
func (r *Renderer) drawMachinePuff(screen *ebiten.Image, now uint64, b *game.BuildingView, tile float64) {
	steam := b.Kind == geothermalKind
	part, cs, k := "smoke", smokeCS, smokeScale
	if steam {
		part, cs, k = "steam", noCS, geoSteamScale
	}
	key := buildSprite(b.Kind, b.Facing, 0, false, false)
	x, y, ok := attachWorld(r.art, key, part, float64(b.X), float64(b.Y))
	if !ok || !r.onScreenTile(screen, b.X, b.Y, tile) {
		return
	}
	f, a, ok := machinePuff(now, uint64(b.ID), steam)
	if !ok {
		return
	}
	if r.reducedMotion {
		f = 1
	}
	sx, sy := r.worldToScreenF(x, y)
	if r.drawPuff(screen, f, sx, sy, tile, k, colorScale{cs[0], cs[1], cs[2], cs[3] * float32(a)}) {
		r.powerTrace(part, key)
	}
}

// coveredVent reports a vent under one of covers (x, y, w, h).
func coveredVent(covers [][4]int, b *game.BuildingView) bool {
	for _, c := range covers {
		if b.X < c[0]+c[2] && c[0] < b.X+max(b.W, 1) && b.Y < c[1]+c[3] && c[1] < b.Y+max(b.H, 1) {
			return true
		}
	}
	return false
}

// ventTraceKey is a vent puff's trace key: the frame's sprite @x,y.
func ventTraceKey(f, x, y int) string {
	return "effect/steam_puff_" + strconv.Itoa(f) + "@" + strconv.Itoa(x) + "," + strconv.Itoa(y)
}
