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

// spatial places a world point (tiles) for the ear: pan from where it is
// across the view, volume falling off to silence half a view beyond the
// edge, and quieter when zoomed out. ok is false when it is out of earshot.
func spatial(r *Renderer, x, y float64, v viewport) (vol, pan float64, ok bool) {
	sx, sy := r.worldToScreenF(x, y)
	fx, fy := float64(sx), float64(sy)
	cx := (v.x0 + v.x1) / 2
	half := v.w() / 2
	if half <= 0 || v.h() <= 0 {
		return 0, 0, false
	}
	pan = math.Max(-1, math.Min(1, (fx-cx)/half)) * panWidth
	dx := math.Max(0, math.Max(v.x0-fx, fx-v.x1)) / half
	dy := math.Max(0, math.Max(v.y0-fy, fy-v.y1)) / (v.h() / 2)
	vol = 1 - math.Max(dx, dy)
	if vol <= 0 {
		return 0, 0, false
	}
	vol *= math.Max(0.35, math.Min(1, 0.35+0.65*r.cam.zoom))
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
		if count[c.id] >= cueMaxPerID {
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
