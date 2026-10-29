package overlay

import (
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
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
//   - Conduits have their own pass (DrawConduits) over the floors and
//     under zones and objects, linked on the power layer of linkgrid.go;
//     hidden conduits draw nothing there, only in the power overlay
//     (DrawPowerMarkings, called by POWERUI's power_overlay.go).
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
	ConduitKind       = "power conduit"
	HiddenConduitKind = "hidden conduit"
	PowerSwitchKind   = "power switch"
	WoodGeneratorKind = "wood-fired generator"
	SolarPanelKind    = "solar panel"
	WindTurbineKind   = "wind turbine"
	GeothermalKind    = "geothermal generator"
	BatteryKind       = "battery"
	TorchLampKind     = "torch lamp"
	HeaterKind        = "heater"
	CoolerKind        = "cooler"
)

// IsConduitKind reports a conduit, shown or hidden (the conduit pass).
func IsConduitKind(kind string) bool { return kind == ConduitKind || kind == HiddenConduitKind }

// PowerDeviceKinds are the kinds that make, draw or store power: a
// conduit's stub reaches into them. Built devices also count by their
// view's PowerRole; frames and ghosts, which have none, by this list.
var PowerDeviceKinds = map[string]bool{
	"lamp": true, HeaterKind: true, CoolerKind: true, BatteryKind: true,
	WoodGeneratorKind: true, SolarPanelKind: true, WindTurbineKind: true, GeothermalKind: true,
	"electric stove": true, "electric tailoring bench": true, "electric research bench": true,
}

// LinkKinds is the renderer's sprite.Kinds: BuildingLink and PowerLink.
type LinkKinds struct{}

// Building is BuildingLink.
func (LinkKinds) Building(kind string) sprite.LinkCell { return BuildingLink(kind) }

// Power is PowerLink.
func (LinkKinds) Power(kind string) sprite.LinkCell { return PowerLink(kind) }

// BuildingLink is the link cell of a building or frame kind.
func BuildingLink(kind string) sprite.LinkCell {
	switch kind {
	case "wall", "door", CoolerKind: // a cooler fills a wall gap like a door
		return sprite.WallLink
	case sprite.FenceKind, sprite.GateKind:
		return sprite.FenceLink
	}
	return sprite.LinkCell{}
}

// PowerLink is the power-layer link cell of a building or frame kind:
// conduits link, a switch is a transmitter they join, a device is joined.
func PowerLink(kind string) sprite.LinkCell {
	switch {
	case IsConduitKind(kind):
		return sprite.ConduitLink
	case kind == PowerSwitchKind:
		return sprite.LinkCell{Flags: sprite.LinkConduit}
	case PowerDeviceKinds[kind]:
		return sprite.LinkCell{Flags: sprite.LinkDevice}
	}
	return sprite.LinkCell{}
}

// PowerDraw is the power layer's presentation state (Wiring.Parts):
// each turbine's blade phase, the glow key memo and the reused vent
// cover list. trace, when set (tests), sees every part drawn.
type PowerDraw struct {
	Spins  map[entity.EntityID]turbineSpin
	pruned uint64
	glow   map[string]string
	Covers [][4]int
	Trace  func(part, key string)
}

// PowerTrace reports a part to the test hook.
func (o *Wiring) PowerTrace(part, key string) {
	if o.Parts.Trace != nil {
		o.Parts.Trace(part, key)
	}
}

// GlowKey is the glow layer of a sprite key: <name>_glow, or for a faced
// sprite <name>_glow_<facing> (the art's naming; TestPowerArtCoversRenderer
// checks it against every base's metadata "glow:").
func GlowKey(key string) string {
	for f := art.FaceSouth; f <= art.FaceWest; f++ {
		if base, ok := strings.CutSuffix(key, "_"+f.String()); ok {
			return base + "_glow_" + f.String()
		}
	}
	return key + "_glow"
}

// glowOf is GlowKey memoised (one string per sprite, not per frame).
func (p *PowerDraw) glowOf(key string) string {
	if g, ok := p.glow[key]; ok {
		return g
	}
	if p.glow == nil {
		p.glow = map[string]string{}
	}
	g := GlowKey(key)
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

// OwnsFlame reports a burning kind whose fire power_draw.go draws (the
// torch's flame) or that shows none (the generator's firebox glows and
// smokes): DrawFires and the campfire embers leave them alone.
func OwnsFlame(kind string) bool { return kind == TorchLampKind || kind == WoodGeneratorKind }

// attachWorld is attachment name of sprite key drawn for a building
// anchored at (x, y), in world tiles: placement puts the sprite's
// origin_px on the anchor (artOrigin). ok is false without it. It reads
// the metadata only, never the pixels.
func attachWorld(c *art.Catalog, key, name string, x, y float64) (float64, float64, bool) {
	if c == nil {
		return 0, 0, false
	}
	sp := c.Sprites[key]
	if sp == nil {
		return 0, 0, false
	}
	a, ok := sp.Meta.Attach[name]
	if !ok {
		return 0, 0, false
	}
	return x + (a[0]-sp.Meta.Origin[0])/art.TilePx, y + (a[1]-sp.Meta.Origin[1])/art.TilePx, true
}

// drawUnlitSprite draws key unlit at world (wx, wy), multiplied by cs:
// glow layers and overlay markings, which give their own light. false
// when nothing was drawn (no art, off screen).
func (o *Wiring) drawUnlitSprite(v *render.View, dst *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale) bool {
	sp := v.Art.Sprite(key)
	if sp == nil {
		return false
	}
	sx, sy := v.WorldToScreenF(wx, wy)
	x, y, s := art.Origin(sp.Meta, float64(sx), float64(sy), tile)
	if !sprite.Visible(float32(x), float32(y), float32(float64(sp.Meta.Canvas[0])*s), float32(float64(sp.Meta.Canvas[1])*s), sw, sh) {
		return false
	}
	img := v.Art.Image(key, color.NRGBA{})
	if img == nil {
		return false
	}
	var geo ebiten.GeoM
	geo.Scale(s, s)
	geo.Translate(x, y)
	drawUnlit(dst, img, geo, cs)
	return true
}

// DrawPowerParts draws what a building's power state adds over its
// sprite (key, just drawn): the glow layer while it runs, then the
// turbine's blades, the torch's flame or the battery's charge bar.
func (o *Wiring) DrawPowerParts(v *render.View, ps sprite.Pass, ch *Chunks, dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	if ch.Baking || v.Art == nil || (b.PowerRole == "" && b.Kind != TorchLampKind) {
		return
	}
	if poweredLook(b) {
		if g := o.Parts.glowOf(key); v.Art.Has(g) && o.drawUnlitSprite(v, dst, g, float64(b.X), float64(b.Y), tile, sw, sh, sprite.NoCS) {
			o.PowerTrace("glow", g)
		}
	}
	switch b.Kind {
	case TorchLampKind:
		if torchBurning(b) {
			o.drawTorchFlame(v, dst, b, tile)
		}
	case WindTurbineKind:
		o.drawTurbineBlades(v, ps, dst, b, key, tile, sw, sh)
	case BatteryKind:
		o.drawBatteryBar(v, dst, b, key, tile)
	}
}

// --- conduits ----------------------------------------------------------

// DrawConduits is the conduit pass: over the floors, under zones and
// objects, every shown conduit of the chunks in view, linked to the
// transmitters and devices beside it (overlay.Links.conduitMask), then the
// conduit blueprints (linked with the planned set too) and their
// deconstruct marks. A hidden conduit draws only its blueprint and mark
// here; built, it shows in the power overlay alone. Not on the
// whole-map bake (its tiles are a few pixels wide).
func (o *Wiring) DrawConduits(v *render.View, ps sprite.Pass, ch *Chunks, links *Links, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if ch.WholeMap || v.Art == nil {
		return
	}
	none := color.NRGBA{}
	traced, planned := false, false
	var p map[[2]int]sprite.LinkFlag
	for _, cv := range ch.Views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !IsConduitKind(b.Kind) || !ch.InView(b.X, b.Y, 1, 1) {
				continue
			}
			if !traced {
				traced = true
				v.LayerDrawn("conduits")
			}
			if b.Kind == ConduitKind {
				if key := o.Host.BuildKey(b.Kind, "", links.ConduitMask(b.X, b.Y), false, false); v.Art.Has(key) {
					o.PowerTrace("conduit", key)
					o.P.DrawCS(v, ps, screen, key, none, float64(b.X), float64(b.Y), tile, sw, sh, sprite.NoCS)
				}
			}
			if b.Deconstruct {
				o.P.Draw(v, ps, screen, "overlay/designation_deconstruct", none, float64(b.X), float64(b.Y), tile, sw, sh)
			}
		}
		for _, i := range cv.Frames {
			if int(i) >= len(w.Frames) {
				continue
			}
			f := &w.Frames[i]
			if !IsConduitKind(f.Kind) || !ch.InView(f.X, f.Y, 1, 1) {
				continue
			}
			if !planned { // the frames' planned set, once (O(frames))
				planned = true
				p = links.Plan(w.Frames, nil)
			}
			key := o.Host.BuildKey(f.Kind, "", links.PowerMask(sprite.ConduitLink.Accepts, f.X, f.Y, p), false, false)
			if v.Art.Has(key) {
				o.PowerTrace("conduit frame", key)
				o.P.DrawCS(v, ps, screen, key, none, float64(f.X), float64(f.Y), tile, sw, sh, sprite.BlueprintCS)
			}
		}
	}
}

// DrawPowerMarkings draws the power overlay's conduit markings (POWERUI's
// power_overlay.go calls it after the world pass): the hidden-conduit
// marking of every conduit in view (only the hidden ones when
// hiddenOnly), linked like the conduits, multiplied by colour(its net)
// and unlit, so a net reads as one colour by night too.
func (o *Wiring) DrawPowerMarkings(v *render.View, ch *Chunks, links *Links, dst *ebiten.Image, w *game.WorldView, tile float64, sw, sh int, colour func(net int) color.NRGBA, hiddenOnly bool) {
	if v.Art == nil || colour == nil {
		return
	}
	links.Sync(*w, v.MapW, v.MapH) // a no-op on the frame's StructVer
	for _, cv := range ch.Views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !IsConduitKind(b.Kind) || (hiddenOnly && b.Kind != HiddenConduitKind) || !ch.InView(b.X, b.Y, 1, 1) {
				continue
			}
			key := o.Host.BuildKey(HiddenConduitKind, "", links.ConduitMask(b.X, b.Y), false, false)
			c := colour(b.PowerNet)
			cs := sprite.ColorScale{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
			if o.drawUnlitSprite(v, dst, key, float64(b.X), float64(b.Y), tile, sw, sh, cs) {
				o.PowerTrace("marking", key)
			}
		}
	}
}

// --- wind turbine -----------------------------------------------------

// The blade frames (metadata frames and step_deg): eight frames 15°
// apart loop a three-bladed rotor through a third of a turn.
const (
	TurbineFrames  = 8
	TurbineStepDeg = 15
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

// TurbineFPS is how fast the blades of turbine b turn in the wind
// (windX, windY tiles per second): 0 while it is switched off (PowerOn is
// the actual state) or the air is calm.
func TurbineFPS(windX, windY float64, b *game.BuildingView) float64 {
	s := math.Hypot(windX, windY)
	if !b.PowerOn || s < turbineCalm {
		return 0
	}
	return math.Min(s*turbineFPSPerWind, turbineMaxFPS)
}

// BladeFrame advances turbine id's phase to present frame now at fps
// frames per second and returns its blade frame (1..turbineFrames). The
// phase accumulates, so a change of wind changes the speed, never the
// angle; a new turbine starts at a phase of its own.
func (p *PowerDraw) BladeFrame(id entity.EntityID, fps float64, now uint64) int {
	if p.Spins == nil {
		p.Spins = map[entity.EntityID]turbineSpin{}
	}
	s, ok := p.Spins[id]
	switch {
	case !ok:
		s = turbineSpin{phase: float64(art.Mix64(uint64(id)) % TurbineFrames), last: now}
	case now > s.last:
		s.phase = math.Mod(s.phase+float64(now-s.last)*fps/60, TurbineFrames)
		s.last = now
	}
	p.Spins[id] = s
	return int(s.phase)%TurbineFrames + 1
}

// Prune forgets the turbines not drawn for turbineForget frames (gone or
// long off screen), at most once per turbineForget frames.
func (p *PowerDraw) Prune(now uint64) {
	if now-p.pruned < turbineForget {
		return
	}
	p.pruned = now
	for id, s := range p.Spins {
		if now-s.last > turbineForget {
			delete(p.Spins, id)
		}
	}
}

// BladeKey is the blade frame's sprite, power/wind_turbine_blades_NN.
func BladeKey(frame int) string { return "power/wind_turbine_blades_" + sprite.TwoDigits(frame) }

// BladeAt is where the blade sprite bladeK is anchored so its hub lands
// on the rotor of turbine b drawn as baseKey.
func (o *Wiring) BladeAt(v *render.View, b *game.BuildingView, baseKey, bladeK string) (float64, float64, bool) {
	rx, ry, ok := attachWorld(v.Art, baseKey, "rotor", float64(b.X), float64(b.Y))
	if !ok {
		return 0, 0, false
	}
	bl := v.Art.Sprites[bladeK]
	if bl == nil {
		return 0, 0, false
	}
	hub, ok := bl.Meta.Attach["hub"]
	if !ok {
		return 0, 0, false
	}
	return rx - (hub[0]-bl.Meta.Origin[0])/art.TilePx, ry - (hub[1]-bl.Meta.Origin[1])/art.TilePx, true
}

// drawTurbineBlades draws turbine b's blades over its base (key), on the
// frame its phase has reached; Reduced Motion holds them still.
func (o *Wiring) drawTurbineBlades(v *render.View, ps sprite.Pass, dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	fps := TurbineFPS(v.W.WindX, v.W.WindY, b)
	if v.ReducedMotion {
		fps = 0
	}
	k := BladeKey(o.Parts.BladeFrame(b.ID, fps, v.Present))
	wx, wy, ok := o.BladeAt(v, b, key, k)
	if !ok {
		return
	}
	o.PowerTrace("blades", k)
	o.P.DrawCS(v, ps, dst, k, color.NRGBA{}, wx, wy, tile, sw, sh, sprite.NoCS)
}

// --- torch lamp --------------------------------------------------------

// TorchFlameScale is the torch's flame size against the campfire's
// (metadata flame_scale; TestPowerArtCoversRenderer keeps them equal).
const TorchFlameScale = 0.55

// torchBurning reports a torch lamp on fire: the sim says Burning, or it
// gives light (Glow). One with fuel the rain put out is dark.
func torchBurning(b *game.BuildingView) bool { return b.Burning || b.Glow }

// TorchFireAt is where torch b, drawn as key, has its flame: its sprite's fire
// attachment (ok false without one).
func TorchFireAt(c *art.Catalog, key string, b *game.BuildingView) (float64, float64, bool) {
	return attachWorld(c, key, "fire", float64(b.X), float64(b.Y))
}

// drawTorchFlame draws the campfire flame, scaled down, on the torch's
// head (its tile's top centre without the attachment).
func (o *Wiring) drawTorchFlame(v *render.View, dst *ebiten.Image, b *game.BuildingView, tile float64) {
	key := o.Host.BuildKey(TorchLampKind, "", 0, false, false)
	x, y, ok := TorchFireAt(v.Art, key, b)
	if !ok {
		x, y = float64(b.X)+0.5, float64(b.Y)
	}
	o.PowerTrace("flame", key)
	o.drawFlame(v, dst, x, y, float64(v.Present)+float64(b.ID)*17, tile, TorchFlameScale)
}

// --- battery -----------------------------------------------------------

// batteryBarPx is the charge bar's thickness in canvas pixels (the gauge
// slot the art leaves between gauge0 and gauge1 is about 12).
const batteryBarPx = 9.0

// BatteryFill is battery b's charge, 0..1.
func BatteryFill(b *game.BuildingView) float64 {
	if b.CapWh <= 0 || b.StoredWh <= 0 {
		return 0
	}
	return math.Min(float64(b.StoredWh)/float64(b.CapWh), 1)
}

// BatteryBarRect is the screen rectangle of a bar th thick that runs
// from (x0, y0), empty, toward (x1, y1), full, filled f of the way. The
// gauges lie along one axis (vertical facing south or north, horizontal
// east or west).
func BatteryBarRect(x0, y0, x1, y1, f, th float64) (x, y, w, h float64) {
	ex, ey := x0+(x1-x0)*f, y0+(y1-y0)*f
	if math.Abs(x1-x0) >= math.Abs(y1-y0) {
		return math.Min(x0, ex), y0 - th/2, math.Abs(ex - x0), th
	}
	return x0 - th/2, math.Min(y0, ey), th, math.Abs(ey - y0)
}

// BatteryBarColour is green, amber below a quarter.
func BatteryBarColour(f float64) color.NRGBA {
	if f < 0.25 {
		return color.NRGBA{R: 226, G: 170, B: 64, A: 235}
	}
	return color.NRGBA{R: 104, G: 200, B: 112, A: 235}
}

// drawBatteryBar draws battery b's charge in its gauge slot, unlit (an
// indicator), from gauge0 toward gauge1.
func (o *Wiring) drawBatteryBar(v *render.View, dst *ebiten.Image, b *game.BuildingView, key string, tile float64) {
	f := BatteryFill(b)
	if f <= 0 {
		return
	}
	x0, y0, ok0 := attachWorld(v.Art, key, "gauge0", float64(b.X), float64(b.Y))
	x1, y1, ok1 := attachWorld(v.Art, key, "gauge1", float64(b.X), float64(b.Y))
	if !ok0 || !ok1 {
		return
	}
	sx0, sy0 := v.WorldToScreenF(x0, y0)
	sx1, sy1 := v.WorldToScreenF(x1, y1)
	x, y, w, h := BatteryBarRect(float64(sx0), float64(sy0), float64(sx1), float64(sy1), f, batteryBarPx*tile/art.TilePx)
	o.PowerTrace("battery", key)
	o.solidRect(dst, float32(x), float32(y), float32(w), float32(h), BatteryBarColour(f))
}

// --- no-power badge -----------------------------------------------------

// NoPowerIcon is the badge over an unpowered consumer.
const NoPowerIcon = "ui_icon/no_power"

// The badge: badgeTiles of a tile wide, raised badgeLift tiles above its
// footprint's centre, its alpha easing between 1 and 1-badgeDip over
// badgePeriod present frames.
const (
	badgeTiles  = 0.42
	badgeLift   = 0.35
	badgeDip    = 0.55
	badgePeriod = 90
)

// NoPowerBadge reports a consumer that wants power and has none (the
// player switching it off is no fault; producers have their own lines).
func NoPowerBadge(b *game.BuildingView) bool {
	return b.PowerRole == "consumer" && b.NoPowerReason != "" && b.NoPowerReason != game.NoPowerSwitchedOff
}

// BadgeAlpha is the badge's soft blink at present frame now: a cosine
// ease between 1 and 1-badgeDip, steady at 1 under Reduced Flashing.
func BadgeAlpha(now uint64, reducedFlashing bool) float64 {
	if reducedFlashing {
		return 1
	}
	t := float64(now%badgePeriod) / badgePeriod
	return 1 - badgeDip*(0.5-0.5*math.Cos(2*math.Pi*t))
}

// DrawNoPowerBadges draws the badge over every unpowered consumer in
// view, after the objects so nothing hides it. Not on the whole-map
// bake.
func (o *Wiring) DrawNoPowerBadges(v *render.View, ch *Chunks, screen *ebiten.Image, w *game.WorldView, tile float64) {
	if v.Art == nil || ch.WholeMap {
		return
	}
	var img *ebiten.Image
	a := float32(BadgeAlpha(v.Present, v.ReducedFlashing))
	size := tile * badgeTiles // physical, like the anchor
	for _, cv := range ch.Views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			b := &w.Buildings[i]
			if !NoPowerBadge(b) || !ch.InView(b.X, b.Y, 1, 1) {
				continue
			}
			if img == nil {
				if img = v.Art.Image(NoPowerIcon, color.NRGBA{}); img == nil {
					return
				}
			}
			sx, sy := v.WorldToScreenF(float64(b.X)+float64(max(b.W, 1))/2, float64(b.Y)+float64(max(b.H, 1))/2-badgeLift)
			o.PowerTrace("badge", NoPowerIcon)
			worldIconCS(screen, img, float64(sx)-size/2, float64(sy)-size/2, size/gfx.UIScale(), sprite.ColorScale{a, a, a, a})
		}
	}
}

// worldIconCS is worldIcon (ui_canvas.go) multiplied by cs: a physical
// anchor, a logical size, converted once.
func worldIconCS(dst, img *ebiten.Image, physX, physY, size float64, cs sprite.ColorScale) {
	k := 1 / gfx.UIScale()
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
var smokeCS = sprite.ColorScale{0.4, 0.38, 0.36, 0.8}

// machinePuff is the puff frame and alpha of machine id at tick now (ok
// false between puffs), steam or smoke timing, offset by its id.
func (o *Wiring) machinePuff(now, id uint64, steam bool) (int, float64, bool) {
	h := art.Mix64(id ^ 0x5e4a9)
	lo, spread := uint64(smokePeriodMin), uint64(smokePeriodSpread)
	if steam {
		lo, spread = geoPeriodMin, geoPeriodSpread
	}
	period := lo + h>>8%(spread+1)
	return o.Host.PuffAt((now + h>>24%period) % period)
}

// DrawMachinePuff draws running generator b's exhaust at its smoke or
// steam attachment.
func (o *Wiring) DrawMachinePuff(v *render.View, screen *ebiten.Image, now uint64, b *game.BuildingView, tile float64) {
	steam := b.Kind == GeothermalKind
	part, cs, k := "smoke", smokeCS, smokeScale
	if steam {
		part, cs, k = "steam", sprite.NoCS, geoSteamScale
	}
	key := o.Host.BuildKey(b.Kind, b.Facing, 0, false, false)
	x, y, ok := attachWorld(v.Art, key, part, float64(b.X), float64(b.Y))
	if !ok || !onScreenTile(v, screen, b.X, b.Y, tile) {
		return
	}
	f, a, ok := o.machinePuff(now, uint64(b.ID), steam)
	if !ok {
		return
	}
	if v.ReducedMotion {
		f = 1
	}
	sx, sy := v.WorldToScreenF(x, y)
	if o.Host.Puff(screen, f, sx, sy, tile, k, sprite.ColorScale{cs[0], cs[1], cs[2], cs[3] * float32(a)}) {
		o.PowerTrace(part, key)
	}
}

// CoveredVent reports a vent under one of covers (x, y, w, h).
func CoveredVent(covers [][4]int, b *game.BuildingView) bool {
	for _, c := range covers {
		if b.X < c[0]+c[2] && c[0] < b.X+max(b.W, 1) && b.Y < c[1]+c[3] && c[1] < b.Y+max(b.H, 1) {
			return true
		}
	}
	return false
}

// VentTraceKey is a vent puff's trace key: the frame's sprite @x,y.
func VentTraceKey(f, x, y int) string {
	return "effect/steam_puff_" + strconv.Itoa(f) + "@" + strconv.Itoa(x) + "," + strconv.Itoa(y)
}
