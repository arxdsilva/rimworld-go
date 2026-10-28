package ebiten

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// The sound director (SND-001) decides, once per frame, what the player
// hears: the colony's cues placed in stereo by where they are on screen,
// ambience from the time of day and the weather, campfires, and the
// music for the moment. It only talks to the Sound interface (sound.go).

const (
	cueGap        = 4       // frames between plays of one sound
	cueFastNear   = 4       // at the fastest speed, only the cues nearest the centre
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
	Frame uint64
	last  map[string]uint64 // frame each id last played
	loops map[string]string // live loop emitter -> sound id
}

// play rate-limits one sound id.
func (a *App) play(id string, vol, pan float64) {
	d := &a.snd
	if last, ok := d.last[id]; ok && d.Frame-last < cueGap {
		return
	}
	if d.last == nil {
		d.last = map[string]uint64{}
	}
	d.last[id] = d.Frame
	a.cfg.Sound.Play(id, vol, pan)
}

// uiSound plays an interface sound for a click on b.
func (a *App) uiSound(b button) {
	switch {
	case b.Disabled:
		a.cfg.Sound.Play("ui.deny", 1, 0)
	case strings.HasPrefix(b.ID, "options.") && b.ID != "options.back":
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
	if sp.Discrete() {
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
	d.Frame++
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
			want["menu:night"] = loopWant{ID: "amb.night", Vol: menuNightVol}
		}
		want["menu:fire"] = loopWant{ID: "amb.campfire", Vol: menuFireVol}
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
		if !snd.Has(w.ID) {
			continue
		}
		snd.Loop(key, w.ID, w.Vol, w.Pan)
		d.loops[key] = w.ID
	}
}

// colonyCues plays this frame's renderer cues: global ones as they come,
// placed ones by where they are, at most cueMaxPerID of a sound, and at
// the fastest speed only the nearest few.
func (a *App) colonyCues() {
	r := a.renderer
	v := a.mapViewport() // physical, like worldToScreen
	_, _, perID := zoomEar(r.cam.Zoom, r.MinZoomFor(v))
	type placed struct {
		id       string
		vol, pan float64
		dist     float64
	}
	var near []placed
	for _, c := range r.takeCues() {
		if c.Global {
			a.play(c.ID, c.Vol, 0)
			continue
		}
		vol, pan, ok := spatial(r, c.X, c.Y, v)
		if !ok {
			continue
		}
		sx, sy := r.worldToScreenF(c.X, c.Y)
		dist := math.Hypot(float64(sx)-(v.X0+v.X1)/2, float64(sy)-(v.Y0+v.Y1)/2)
		near = append(near, placed{c.ID, vol * c.Vol, pan, dist})
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
	v := a.mapViewport() // physical, like worldToScreen
	rain := 0.0
	switch w.Weather {
	case "rain":
		rain = w.WeatherIntensity
	case "storm":
		rain = math.Min(1, 0.3+w.WeatherIntensity)
	}
	inside := 1.0
	if wx, wy := r.screenToWorld(int((v.X0+v.X1)/2), int((v.Y0+v.Y1)/2)); w.TerrainW > 0 {
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
	want["amb.day"] = loopWant{ID: "amb.day", Vol: day * (1 - 0.5*rain) * birds * inside}
	crickets := 1.0
	if winter || w.OutdoorTemp <= 80 {
		crickets = 0
	}
	want["amb.night"] = loopWant{ID: "amb.night", Vol: (1 - day) * (1 - 0.5*rain) * crickets * inside}
	want["amb.rain"] = loopWant{ID: "amb.rain", Vol: rain * inside}
	water, shore := r.waterOnScreen(w, v)
	want["amb.water_river"] = loopWant{ID: "amb.water_river", Vol: math.Min(1, water*waterRiverGain) * inside}
	want["amb.water_shore"] = loopWant{ID: "amb.water_shore", Vol: math.Min(1, shore*waterShoreGain) * inside}
	type fire struct {
		key      string
		vol, pan float64
	}
	var fires []fire
	for _, b := range w.Buildings {
		if !b.Burning || b.Kind == sndWoodGenerator { // a generator has its own chug
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
		want[f.key] = loopWant{ID: "amb.campfire", Vol: f.vol, Pan: f.pan}
	}
	r.blazeAmbience(want, w, v)
	r.machineAmbience(want, w, v)
	for key, l := range want { // drop silent beds instead of keeping them at 0
		if l.Vol <= 0.001 {
			delete(want, key)
		}
	}
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
