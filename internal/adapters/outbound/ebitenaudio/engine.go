// Package ebitenaudio plays the game's sound list
// (assets/metadata/audio/audio.json) through Ebitengine's audio package:
// one-shots, positional loops, and music with crossfades and silences.
package ebitenaudio

import (
	"fmt"
	"io"
	"log"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"rimworld-go/internal/platform/config"
	"rimworld-go/pkg/audiomanifest"
)

const (
	tps      = 60  // Update calls per second
	maxShots = 24  // one-shots playing at once; the oldest makes room
	loopFade = 0.8 // seconds for a loop to fade fully in or out
	musicXF  = 2.5 // seconds for a music crossfade
	gapMin   = 40  // seconds of silence between non-looping music plays
	gapMax   = 150 //
)

// Engine is the game's sound (it satisfies the client's Sound port). All
// methods run on the game loop; only a loop's pan is shared with the
// audio goroutine.
type Engine struct {
	entries map[string]audiomanifest.Entry
	pcm     map[string][][]int16 // WAV variants, mono 48 kHz
	ogg     map[string][][]byte  // OGG variants, encoded

	b       backend
	decode  func([]byte) (io.ReadSeeker, int64, error)
	focused func() bool

	vol    config.Audio
	rng    splitmix
	last   map[string]int // previous variant per id
	warned map[string]bool

	shots   []*shot
	loops   map[string]*loopVoice
	loopSeq int
	music   *track
	fading  []*track
}

type shot struct {
	p    player
	bus  string
	base float64 // cue volume × entry volume
}

type loopVoice struct {
	id          string
	e           audiomanifest.Entry
	p           player
	pan         *loopReader // nil for OGG loops (pan ignored)
	cur, target float64
}

type track struct {
	id          string
	e           audiomanifest.Entry
	p           player // nil while silent between plays
	cur, target float64
	next        int // next file index
	wait        int // frames of silence left before the next play
}

// New reads the sound list and its files under assetsRoot, then opens the
// audio device. A bad sound list is an error (and no device is opened);
// missing or unreadable files are logged and skipped.
func New(assetsRoot string) (*Engine, error) {
	m, pcm, ogg, err := load(assetsRoot)
	if err != nil {
		return nil, err
	}
	e := newEngine(m, pcm, ogg, newDevice())
	e.rng.s = uint64(time.Now().UnixNano())
	return e, nil
}

func newEngine(m audiomanifest.Manifest, pcm map[string][][]int16, ogg map[string][][]byte, b backend) *Engine {
	e := &Engine{
		entries: map[string]audiomanifest.Entry{},
		pcm:     pcm,
		ogg:     ogg,
		b:       b,
		decode:  decodeOGG,
		focused: ebiten.IsFocused,
		vol:     config.Defaults().Audio,
		rng:     splitmix{s: 1},
		last:    map[string]int{},
		warned:  map[string]bool{},
		loops:   map[string]*loopVoice{},
	}
	if e.pcm == nil {
		e.pcm = map[string][][]int16{}
	}
	if e.ogg == nil {
		e.ogg = map[string][][]byte{}
	}
	for _, en := range m.Sounds {
		e.entries[en.ID] = en
	}
	return e
}

// Has reports whether id is in the sound list with at least one playable
// file.
func (e *Engine) Has(id string) bool {
	_, ok := e.entries[id]
	return ok && e.variants(id) > 0
}

// Play starts a one-shot of id at vol (0..1) and pan (-1..1).
func (e *Engine) Play(id string, vol, pan float64) {
	en, ok := e.entry(id)
	if !ok || vol <= 0 {
		return
	}
	p, _, err := e.open(id, e.pick(id), false, pan)
	if err != nil {
		e.warn(id, "%v", err)
		return
	}
	if len(e.shots) >= maxShots {
		_ = e.shots[0].p.Close()
		e.shots = append(e.shots[:0], e.shots[1:]...)
	}
	s := &shot{p: p, bus: en.Bus, base: clamp(vol, 0, 1) * en.Volume}
	p.SetVolume(e.gain(s.bus, s.base))
	p.Play()
	e.shots = append(e.shots, s)
}

// Loop keeps emitter key playing id, fading toward vol and panned to pan;
// vol 0 fades it out and frees it. A new id on the same key fades the old
// sound out and starts the new one.
func (e *Engine) Loop(key, id string, vol, pan float64) {
	vol = clamp(vol, 0, 1)
	l := e.loops[key]
	if l != nil && l.id != id {
		l.target = 0
		e.loopSeq++
		e.loops[fmt.Sprintf("%s\x00%d", key, e.loopSeq)] = l
		delete(e.loops, key)
		l = nil
	}
	if l == nil {
		if vol == 0 {
			return
		}
		en, ok := e.entry(id)
		if !ok {
			return
		}
		p, lr, err := e.open(id, e.pick(id), true, pan)
		if err != nil {
			e.warn(id, "%v", err)
			return
		}
		p.SetVolume(0)
		p.Play()
		l = &loopVoice{id: id, e: en, p: p, pan: lr}
		e.loops[key] = l
	}
	l.target = vol
	if l.pan != nil {
		l.pan.pan.set(pan)
	}
}

// Music crossfades to track at vol; "" (or an unknown track) fades the
// current one out.
func (e *Engine) Music(name string, vol float64) {
	vol = clamp(vol, 0, 1)
	if e.music != nil && e.music.id == name {
		e.music.target = vol
		return
	}
	if e.music != nil {
		e.music.target = 0
		e.fading = append(e.fading, e.music)
		e.music = nil
	}
	if name == "" {
		return
	}
	for i, t := range e.fading { // coming back to a track still fading out
		if t.id == name {
			e.fading = append(e.fading[:i], e.fading[i+1:]...)
			t.target = vol
			e.music = t
			return
		}
	}
	en, ok := e.entry(name)
	if !ok {
		return
	}
	t := &track{id: name, e: en, target: vol}
	e.start(t)
	e.music = t
}

// SetVolumes sets the mixer; Update applies it to every live voice.
func (e *Engine) SetVolumes(a config.Audio) { e.vol = a }

// Update runs once per frame: fades, volume changes, finished voices,
// silences between music plays.
func (e *Engine) Update() {
	live := e.shots[:0]
	for _, s := range e.shots {
		if !s.p.IsPlaying() {
			_ = s.p.Close()
			continue
		}
		s.p.SetVolume(e.gain(s.bus, s.base))
		live = append(live, s)
	}
	for i := len(live); i < len(e.shots); i++ {
		e.shots[i] = nil
	}
	e.shots = live

	for key, l := range e.loops {
		l.cur = approach(l.cur, l.target, 1/(loopFade*tps))
		if l.cur == 0 && l.target == 0 {
			_ = l.p.Close()
			delete(e.loops, key)
			continue
		}
		l.p.SetVolume(e.gain(l.e.Bus, l.cur*l.e.Volume))
	}

	if t := e.music; t != nil {
		t.cur = approach(t.cur, t.target, 1/(musicXF*tps))
		switch {
		case t.p != nil && !t.e.Loop && !t.p.IsPlaying():
			_ = t.p.Close()
			t.p = nil
			t.wait = gapMin*tps + e.rng.intn((gapMax-gapMin)*tps+1)
		case t.p != nil:
			t.p.SetVolume(e.gain(t.e.Bus, t.cur*t.e.Volume))
		case t.wait > 0:
			if t.wait--; t.wait == 0 {
				e.start(t)
			}
		}
	}
	keep := e.fading[:0]
	for _, t := range e.fading {
		t.cur = approach(t.cur, 0, 1/(musicXF*tps))
		if t.p != nil && (t.cur == 0 || !t.p.IsPlaying()) {
			_ = t.p.Close()
			t.p = nil
		}
		if t.p == nil {
			continue
		}
		t.p.SetVolume(e.gain(t.e.Bus, t.cur*t.e.Volume))
		keep = append(keep, t)
	}
	for i := len(keep); i < len(e.fading); i++ {
		e.fading[i] = nil
	}
	e.fading = keep
}

// start plays t's next file (looping when the entry loops).
func (e *Engine) start(t *track) {
	n := e.variants(t.id)
	if n == 0 {
		return
	}
	p, _, err := e.open(t.id, t.next%n, t.e.Loop, 0)
	t.next = (t.next + 1) % n
	if err != nil {
		e.warn(t.id, "%v", err)
		return
	}
	p.SetVolume(e.gain(t.e.Bus, t.cur*t.e.Volume))
	p.Play()
	t.p = p
}

// open makes a voice for variant v of id: WAV variants come first, then
// OGG ones. A WAV loop is panned live through the returned reader.
func (e *Engine) open(id string, v int, loop bool, pan float64) (player, *loopReader, error) {
	if wavs := e.pcm[id]; v < len(wavs) {
		if loop {
			lr := newLoopReader(wavs[v], pan)
			p, err := e.b.fromStream(lr)
			return p, lr, err
		}
		return e.b.fromBytes(stereoBytes(wavs[v], pan)), nil, nil
	}
	oggs := e.ogg[id]
	v -= len(e.pcm[id])
	if v < 0 || v >= len(oggs) {
		return nil, nil, fmt.Errorf("no variant %d", v)
	}
	s, n, err := e.decode(oggs[v])
	if err != nil {
		return nil, nil, err
	}
	var r io.Reader = s
	if loop {
		r = audio.NewInfiniteLoop(s, n)
	}
	p, err := e.b.fromStream(r)
	return p, nil, err
}

// entry is id's sound list entry when it has something to play; otherwise
// it logs once.
func (e *Engine) entry(id string) (audiomanifest.Entry, bool) {
	en, ok := e.entries[id]
	switch {
	case !ok:
		e.warn(id, "not in the sound list")
	case e.variants(id) == 0:
		e.warn(id, "no playable files")
		ok = false
	}
	return en, ok
}

func (e *Engine) variants(id string) int { return len(e.pcm[id]) + len(e.ogg[id]) }

// pick is a random variant of id, never the previous one when there are
// several.
func (e *Engine) pick(id string) int {
	n := e.variants(id)
	if n <= 1 {
		return 0
	}
	prev, seen := e.last[id]
	v := e.rng.intn(n)
	if seen {
		v = e.rng.intn(n - 1)
		if v >= prev {
			v++
		}
	}
	e.last[id] = v
	return v
}

// gain is base × the bus volume × master, silenced when unfocused if the
// player asked for that.
func (e *Engine) gain(bus string, base float64) float64 {
	if e.vol.MuteUnfocused && e.focused != nil && !e.focused() {
		return 0
	}
	var b int
	switch bus {
	case audiomanifest.BusMusic:
		b = e.vol.Music
	case audiomanifest.BusAmbience:
		b = e.vol.Ambience
	case audiomanifest.BusUI:
		b = e.vol.UI
	default:
		b = e.vol.Effects
	}
	return clamp(base*config.Gain(b)*config.Gain(e.vol.Master), 0, 1)
}

func (e *Engine) warn(id, format string, args ...any) {
	if e.warned[id] {
		return
	}
	e.warned[id] = true
	log.Printf("sound %s: %s", id, fmt.Sprintf(format, args...))
}

// approach moves v toward target by at most step.
func approach(v, target, step float64) float64 {
	switch {
	case v < target:
		return min(target, v+step)
	case v > target:
		return max(target, v-step)
	}
	return v
}
