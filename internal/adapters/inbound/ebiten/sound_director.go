package ebiten

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
)

// The sound director (SND-001) decides, once per frame, what the player
// hears: the colony's cues placed in stereo by where they are on screen,
// ambience from the time of day and the weather, campfires, and the
// music for the moment. It only talks to the Sound interface (sound.go).

const (
	cueGap        = 4       // frames between plays of one sound
	cueMaxPerID   = 3       // copies of one sound per frame
	cueFastNear   = 4       // at the fastest speed, only the cues nearest the centre
	panWidth      = 0.8     // how far left/right a cue can sit
	tensionHold   = 45 * 60 // frames the tension music holds after a threat
	pausedMusic   = 0.6     // music level under the pause menu
	menuNightVol  = 0.35
	menuFireVol   = 0.5
	indoorDamping = 0.4
	trackCalm     = "music.calm"
	trackTension  = "music.tension"
	trackMenu     = "music.menu"
)

type soundDirector struct {
	frame uint64
	last  map[string]uint64 // frame each id last played
	loops map[string]string // live loop emitter -> sound id
}

type loopWant struct {
	id       string
	vol, pan float64
}

// zoomNear is the zoom at or closer than which effects play at full
// volume and full stereo width.
const zoomNear = 1.5

// zoomEar is how the camera's distance shapes the effects: from zoomNear
// out to the map's zoom-out limit (zoomOut) they get quieter, narrower in
// stereo and fewer per frame, reaching silence fully zoomed out. Each
// halving of the zoom is an equal step.
func zoomEar(zoom, zoomOut float64) (gain, panScale float64, perID int) {
	span := math.Log2(zoomNear / zoomOut)
	if zoom <= 0 || span <= 0 { // the map fits at zoomNear: no room to fade
		return 1, 1, cueMaxPerID
	}
	far := math.Max(0, math.Min(1, math.Log2(zoomNear/zoom)/span))
	gain = 1 - far
	panScale = 1 - 0.5*far
	switch {
	case far > 0.6:
		perID = 1
	case far > 0.3:
		perID = 2
	default:
		perID = cueMaxPerID
	}
	return gain, panScale, perID
}

// spatial places a world point (tiles) for the ear: pan from where it is
// across the view, volume falling off to silence half a view beyond the
// edge, and fading with camera distance (zoomEar) to silence fully zoomed
// out. ok is false when it is out of earshot.
func spatial(r *Renderer, x, y float64, v viewport) (vol, pan float64, ok bool) {
	sx, sy := r.worldToScreenF(x, y)
	fx, fy := float64(sx), float64(sy)
	cx := (v.x0 + v.x1) / 2
	half := v.w() / 2
	if half <= 0 || v.h() <= 0 {
		return 0, 0, false
	}
	gain, panScale, _ := zoomEar(r.cam.zoom, r.minZoomFor(v))
	pan = math.Max(-1, math.Min(1, (fx-cx)/half)) * panWidth * panScale
	dx := math.Max(0, math.Max(v.x0-fx, fx-v.x1)) / half
	dy := math.Max(0, math.Max(v.y0-fy, fy-v.y1)) / (v.h() / 2)
	vol = (1 - math.Max(dx, dy)) * gain
	if vol <= 0.01 {
		return 0, 0, false
	}
	return vol, pan, true
}

// play rate-limits one sound id.
func (a *App) play(id string, vol, pan float64) {
	d := &a.snd
	if last, ok := d.last[id]; ok && d.frame-last < cueGap {
		return
	}
	if d.last == nil {
		d.last = map[string]uint64{}
	}
	d.last[id] = d.frame
	a.cfg.Sound.Play(id, vol, pan)
}

// uiSound plays an interface sound for a click on b.
func (a *App) uiSound(b button) {
	switch {
	case b.disabled:
		a.cfg.Sound.Play("ui.deny", 1, 0)
	case strings.HasPrefix(b.id, "options.") && b.id != "options.back":
		// applyOption plays its own toggle
	default:
		a.cfg.Sound.Play("ui.click", 1, 0)
	}
}

// pageSound is the soft swish of a wizard page turning (Next or Back).
func (a *App) pageSound() { a.cfg.Sound.Play("ui.page", 1, 0) }

// sliderTickSteps is how finely a continuous slider ticks: 20 steps along
// its track when it has no step of its own.
const sliderTickSteps = 20

// sliderCue ticks when a slider moves from old to cur across a step: any
// change of a discrete slider, one tick per sliderTickSteps-th of a
// continuous one (or per sp.Step), so a drag ticks as it moves, not every
// frame.
func (a *App) sliderCue(sp sliderSpec, lo, hi, old, cur float64) {
	if sp.discrete() {
		a.play("ui.slider_tick", 1, 0)
		return
	}
	step := sp.Step
	if step <= 0 {
		step = (hi - lo) / sliderTickSteps
	}
	if step <= 0 || math.Floor((old-lo)/step) == math.Floor((cur-lo)/step) {
		return
	}
	a.play("ui.slider_tick", 1, 0)
}

// soundFrame runs the director once per Update.
func (a *App) soundFrame() {
	d := &a.snd
	d.frame++
	want := map[string]loopWant{}
	snd := a.cfg.Sound
	if a.nav.screen() == screenColony && a.renderer != nil {
		a.colonyCues()
		a.colonyAmbience(want)
		track := trackCalm
		if a.underThreat() && snd.Has(trackTension) {
			track = trackTension
		}
		vol := 1.0
		if a.nav.overlay() == overlayPause {
			vol = pausedMusic
		}
		snd.Music(available(snd, track), vol)
	} else {
		if snd.Has("amb.night") {
			want["menu:night"] = loopWant{id: "amb.night", vol: menuNightVol}
		}
		want["menu:fire"] = loopWant{id: "amb.campfire", vol: menuFireVol}
		track := trackMenu
		if !snd.Has(trackMenu) {
			track = trackCalm
		}
		snd.Music(available(snd, track), 1)
	}
	a.setLoops(want)
	snd.Update()
}

// available is track when it is in the sound list, else "" (silence).
func available(snd Sound, track string) string {
	if snd.Has(track) {
		return track
	}
	return ""
}

// setLoops sends every wanted loop and fades out the ones no longer wanted.
func (a *App) setLoops(want map[string]loopWant) {
	d := &a.snd
	snd := a.cfg.Sound
	for key, id := range d.loops {
		if _, ok := want[key]; !ok {
			snd.Loop(key, id, 0, 0)
		}
	}
	d.loops = map[string]string{}
	for key, w := range want {
		if !snd.Has(w.id) {
			continue
		}
		snd.Loop(key, w.id, w.vol, w.pan)
		d.loops[key] = w.id
	}
}

// colonyCues plays this frame's renderer cues: global ones as they come,
// placed ones by where they are, at most cueMaxPerID of a sound, and at
// the fastest speed only the nearest few.
func (a *App) colonyCues() {
	r := a.renderer
	v := colonyViewport(a.sw, a.sh)
	_, _, perID := zoomEar(r.cam.zoom, r.minZoomFor(v))
	type placed struct {
		id       string
		vol, pan float64
		dist     float64
	}
	var near []placed
	for _, c := range r.takeCues() {
		if c.global {
			a.play(c.id, c.vol, 0)
			continue
		}
		vol, pan, ok := spatial(r, c.x, c.y, v)
		if !ok {
			continue
		}
		sx, sy := r.worldToScreenF(c.x, c.y)
		dist := math.Hypot(float64(sx)-(v.x0+v.x1)/2, float64(sy)-(v.y0+v.y1)/2)
		near = append(near, placed{c.id, vol * c.vol, pan, dist})
	}
	sort.SliceStable(near, func(i, j int) bool { return near[i].dist < near[j].dist })
	if r.speed == speedVeryFast && len(near) > cueFastNear {
		near = near[:cueFastNear]
	}
	count := map[string]int{}
	for _, c := range near {
		if count[c.id] >= perID {
			continue
		}
		count[c.id]++
		if count[c.id] == 1 {
			a.play(c.id, c.vol, c.pan)
		} else { // copies in one frame are separate swings, not throttled
			a.cfg.Sound.Play(c.id, c.vol, c.pan)
		}
	}
}

// dayWeight is 1 by day and 0 by night, crossfading at dawn and dusk.
func dayWeight(hour float64) float64 {
	switch {
	case hour < 5 || hour >= 20.5:
		return 0
	case hour < 6.5:
		return (hour - 5) / 1.5
	case hour >= 19:
		return (20.5 - hour) / 1.5
	}
	return 1
}

// colonyAmbience is the day or night bed, rain, and the nearest campfires.
func (a *App) colonyAmbience(want map[string]loopWant) {
	r := a.renderer
	w := r.heard
	v := colonyViewport(a.sw, a.sh)
	rain := 0.0
	switch w.Weather {
	case "rain":
		rain = w.WeatherIntensity
	case "storm":
		rain = math.Min(1, 0.3+w.WeatherIntensity)
	}
	inside := 1.0
	if wx, wy := r.screenToWorld(int((v.x0+v.x1)/2), int((v.y0+v.y1)/2)); w.TerrainW > 0 {
		x, y := int(wx), int(wy)
		if i := y*w.TerrainW + x; x >= 0 && y >= 0 && x < w.TerrainW && i < len(w.RoomGrid) && w.RoomGrid[i] >= 0 {
			inside = indoorDamping
		}
	}
	day := dayWeight(w.Hour)
	winter := w.Season == "winter"
	birds := 1.0
	if winter {
		birds = 0.6
	}
	want["amb.day"] = loopWant{id: "amb.day", vol: day * (1 - 0.5*rain) * birds * inside}
	crickets := 1.0
	if winter || w.OutdoorTemp <= 80 {
		crickets = 0
	}
	want["amb.night"] = loopWant{id: "amb.night", vol: (1 - day) * (1 - 0.5*rain) * crickets * inside}
	want["amb.rain"] = loopWant{id: "amb.rain", vol: rain * inside}
	water, shore := r.waterOnScreen(w, v)
	want["amb.water_river"] = loopWant{id: "amb.water_river", vol: math.Min(1, water*waterRiverGain) * inside}
	want["amb.water_shore"] = loopWant{id: "amb.water_shore", vol: math.Min(1, shore*waterShoreGain) * inside}
	type fire struct {
		key      string
		vol, pan float64
	}
	var fires []fire
	for _, b := range w.Buildings {
		if !b.Burning {
			continue
		}
		vol, pan, ok := spatial(r, float64(b.X)+0.5, float64(b.Y)+0.5, v)
		if ok {
			fires = append(fires, fire{"fire:" + strconv.FormatUint(uint64(b.ID), 10), vol, pan})
		}
	}
	sort.Slice(fires, func(i, j int) bool { return fires[i].vol > fires[j].vol })
	for i, f := range fires {
		if i == 2 {
			break
		}
		want[f.key] = loopWant{id: "amb.campfire", vol: f.vol, pan: f.pan}
	}
	for key, l := range want { // drop silent beds instead of keeping them at 0
		if l.vol <= 0.001 {
			delete(want, key)
		}
	}
}

const (
	// waterSamples is the most tiles sampled across the view each way
	// (strided on big views, so the cost never grows with the map).
	waterSamples = 40
	// waterRiverGain and waterShoreGain turn an on-screen fraction into a
	// volume: a sixth of the view in water, or a tenth of it shoreline,
	// is the full bed.
	waterRiverGain = 6.0
	waterShoreGain = 10.0
)

// waterOnScreen is the share of the visible tiles that are water (shallow
// or deep) and the share that are shore: water with land beside it. It
// samples a strided grid of at most waterSamples x waterSamples tiles.
func (r *Renderer) waterOnScreen(w game.WorldView, v viewport) (water, shore float64) {
	if w.TerrainW <= 0 || len(w.Terrain) == 0 {
		return 0, 0
	}
	h := len(w.Terrain) / w.TerrainW
	x0f, y0f := r.screenToWorld(int(v.x0), int(v.y0))
	x1f, y1f := r.screenToWorld(int(v.x1), int(v.y1))
	x0, y0 := max(0, int(math.Floor(x0f))), max(0, int(math.Floor(y0f)))
	x1, y1 := min(w.TerrainW-1, int(math.Ceil(x1f))), min(h-1, int(math.Ceil(y1f)))
	if x1 < x0 || y1 < y0 {
		return 0, 0
	}
	stride := max(1, (max(x1-x0, y1-y0)+waterSamples)/waterSamples)
	isWater := func(x, y int) (wet, ok bool) {
		if x < 0 || y < 0 || x >= w.TerrainW || y >= h {
			return false, false
		}
		return isWaterCode[w.Terrain[y*w.TerrainW+x]], true
	}
	n, wet, edge := 0, 0, 0
	for y := y0; y <= y1; y += stride {
		for x := x0; x <= x1; x += stride {
			n++
			if here, _ := isWater(x, y); !here {
				continue
			}
			wet++
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if nw, in := isWater(x+d[0], y+d[1]); in && !nw {
					edge++
					break
				}
			}
		}
	}
	return float64(wet) / float64(n), float64(edge) / float64(n)
}

// underThreat is a raid in progress or a threat that arrived recently.
func (a *App) underThreat() bool {
	r := a.renderer
	if r.threatAt != 0 && r.present-r.threatAt < tensionHold {
		return true
	}
	for _, p := range r.game.Snapshot() {
		if p.Hostile && !p.Fleeing {
			return true
		}
	}
	return false
}
