package ebitenaudio

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"rimworld-go/internal/platform/config"
	"rimworld-go/pkg/audiomanifest"
)

type fakePlayer struct {
	data    []byte    // fromBytes
	r       io.Reader // fromStream
	playing bool
	plays   int
	vol     float64
	closed  bool
}

func (p *fakePlayer) Play()               { p.playing, p.plays = true, p.plays+1 }
func (p *fakePlayer) IsPlaying() bool     { return p.playing && !p.closed }
func (p *fakePlayer) SetVolume(v float64) { p.vol = v }
func (p *fakePlayer) Close() error        { p.closed = true; return nil }

type fakeBackend struct {
	bytes   []*fakePlayer
	streams []*fakePlayer
}

func (b *fakeBackend) fromBytes(s []byte) player {
	p := &fakePlayer{data: s}
	b.bytes = append(b.bytes, p)
	return p
}

func (b *fakeBackend) fromStream(r io.Reader) (player, error) {
	p := &fakePlayer{r: r}
	b.streams = append(b.streams, p)
	return p, nil
}

func entry(id, bus string, vol float64, loop bool, files ...string) audiomanifest.Entry {
	return audiomanifest.Entry{ID: id, Bus: bus, Volume: vol, Loop: loop, Files: files, Source: "procedural", License: "CC0"}
}

// testEngine has a UI click, a 3-variant effect, a WAV ambience loop, a
// looping OGG ambience bed and two OGG music tracks (one loops, one
// plays its two files in turn). OGG bytes are fake: decode is stubbed.
func testEngine(t *testing.T) (*Engine, *fakeBackend) {
	t.Helper()
	m := audiomanifest.Manifest{Version: 1, Sounds: []audiomanifest.Entry{
		entry("ui.click", audiomanifest.BusUI, 0.5, false, "ui/click.wav"),
		entry("sfx.chop", audiomanifest.BusEffects, 1, false, "a.wav", "b.wav", "c.wav"),
		entry("amb.campfire", audiomanifest.BusAmbience, 0.7, true, "ambience/campfire.wav"),
		entry("amb.day", audiomanifest.BusAmbience, 0.8, true, "ambience/day.ogg"),
		entry("music.calm", audiomanifest.BusMusic, 1, true, "music/calm.ogg"),
		entry("music.suite", audiomanifest.BusMusic, 1, false, "music/s1.ogg", "music/s2.ogg"),
		entry("sfx.missing", audiomanifest.BusEffects, 1, false, "gone.wav"),
	}}
	tone := []int16{1000, -1000, 2000, -2000}
	pcm := map[string][][]int16{
		"ui.click":     {tone},
		"sfx.chop":     {tone, tone, tone},
		"amb.campfire": {tone},
	}
	ogg := map[string][][]byte{
		"amb.day":     {make([]byte, 4096)},
		"music.calm":  {make([]byte, 4096)},
		"music.suite": {make([]byte, 4096), make([]byte, 4096)},
	}
	b := &fakeBackend{}
	e := newEngine(m, pcm, ogg, b)
	e.decode = func(d []byte) (io.ReadSeeker, int64, error) { return bytes.NewReader(d), int64(len(d)), nil }
	e.focused = func() bool { return true }
	e.SetVolumes(config.Audio{Master: 100, Music: 100, Effects: 100, Ambience: 100, UI: 100})
	return e, b
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestGainBusMasterMute(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	focused := true
	e.focused = func() bool { return focused }
	e.SetVolumes(config.Audio{Master: 50, Music: 100, Effects: 100, Ambience: 100, UI: 80})
	e.Play("ui.click", 0.5, 0)
	p := b.bytes[0]
	want := 0.5 * 0.5 * config.Gain(80) * config.Gain(50)
	if !near(p.vol, want) {
		t.Fatalf("gain %v, want %v", p.vol, want)
	}
	e.SetVolumes(config.Audio{Master: 100, UI: 0, Effects: 100})
	e.Update()
	if p.vol != 0 {
		t.Fatalf("UI bus at 0 gave %v", p.vol)
	}
	e.SetVolumes(config.Audio{Master: 100, UI: 100, MuteUnfocused: true})
	e.Update()
	if !near(p.vol, 0.25) {
		t.Fatalf("focused gain %v, want 0.25", p.vol)
	}
	focused = false
	e.Update()
	if p.vol != 0 {
		t.Fatalf("unfocused with mute gave %v", p.vol)
	}
	e.SetVolumes(config.Audio{Master: 100, UI: 100, MuteUnfocused: false})
	e.Update()
	if !near(p.vol, 0.25) {
		t.Fatalf("unfocused without mute gave %v", p.vol)
	}
}

func frames(b []byte) (l, r []int16) {
	for i := 0; i+4 <= len(b); i += 4 {
		l = append(l, int16(binary.LittleEndian.Uint16(b[i:])))
		r = append(r, int16(binary.LittleEndian.Uint16(b[i+2:])))
	}
	return l, r
}

func TestPanBytes(t *testing.T) {
	t.Parallel()
	pcm := []int16{1000, -1000, 32767, -32768}
	for _, c := range []struct {
		pan          float64
		lSilent, rSi bool
	}{{-1, false, true}, {1, true, false}, {0, false, false}} {
		l, r := frames(stereoBytes(pcm, c.pan))
		for i, s := range pcm {
			wantL, wantR := s, s
			if c.lSilent {
				wantL = 0
			}
			if c.rSi {
				wantR = 0
			}
			if l[i] != wantL || r[i] != wantR {
				t.Fatalf("pan %v frame %d: %d/%d, want %d/%d", c.pan, i, l[i], r[i], wantL, wantR)
			}
		}
	}
	l, r := frames(stereoBytes([]int16{1000}, 0.5))
	if l[0] != 500 || r[0] != 1000 {
		t.Fatalf("pan 0.5: %d/%d", l[0], r[0])
	}
}

func TestLoopReaderPansAndWraps(t *testing.T) {
	t.Parallel()
	lr := newLoopReader([]int16{100, 200}, -1)
	buf := make([]byte, 4*5+2) // a partial frame at the end
	if n, err := lr.Read(buf); n != len(buf) || err != nil {
		t.Fatalf("read %d, %v", n, err)
	}
	l, r := frames(buf)
	if want := []int16{100, 200, 100, 200, 100}; !equal(l, want) || !equal(r, make([]int16, 5)) {
		t.Fatalf("left %v right %v", l, r)
	}
	lr.pan.set(1)
	rest := make([]byte, 6)
	_, _ = lr.Read(rest)
	// the half-sent frame (200 at pan -1) finishes, then 100 at pan 1
	if got := int16(binary.LittleEndian.Uint16(rest[0:])); got != 0 {
		t.Fatalf("frame tail right %d", got)
	}
	if l, r := frames(rest[2:]); l[0] != 0 || r[0] != 100 {
		t.Fatalf("after pan 1: %d/%d", l[0], r[0])
	}
}

func equal(a, b []int16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestUnknownAndMissingAreNoOps(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	for _, id := range []string{"sfx.nope", "sfx.missing"} {
		if e.Has(id) {
			t.Fatalf("Has(%s)", id)
		}
		e.Play(id, 1, 0)
		e.Play(id, 1, 0)
		e.Loop("k", id, 1, 0)
		e.Music(id, 1)
	}
	e.Update()
	if len(b.bytes)+len(b.streams) != 0 || len(e.loops) != 0 || e.music != nil {
		t.Fatal("an unknown sound made a voice")
	}
	if !e.Has("ui.click") || !e.Has("music.calm") {
		t.Fatal("known sounds missing")
	}
}

func TestVoiceCapClosesOldest(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	for range maxShots {
		e.Play("sfx.chop", 1, 0)
	}
	for _, p := range b.bytes {
		if p.closed {
			t.Fatal("closed before the cap")
		}
	}
	e.Play("sfx.chop", 1, 0)
	if !b.bytes[0].closed || b.bytes[1].closed || len(e.shots) != maxShots {
		t.Fatalf("25th: oldest closed %v, second closed %v, voices %d", b.bytes[0].closed, b.bytes[1].closed, len(e.shots))
	}
}

func TestUpdateReapsFinished(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	e.Play("ui.click", 1, 0)
	e.Play("ui.click", 1, 0)
	b.bytes[0].playing = false
	e.Update()
	if !b.bytes[0].closed || b.bytes[1].closed || len(e.shots) != 1 {
		t.Fatalf("reap: %v %v %d", b.bytes[0].closed, b.bytes[1].closed, len(e.shots))
	}
}

func TestVariantsDoNotRepeat(t *testing.T) {
	t.Parallel()
	e, _ := testEngine(t)
	prev, used := -1, map[int]bool{}
	for range 200 {
		v := e.pick("sfx.chop")
		if v == prev {
			t.Fatalf("variant %d twice in a row", v)
		}
		prev, used[v] = v, true
	}
	if len(used) != 3 {
		t.Fatalf("variants used %v", used)
	}
}

func TestLoopFadesInAndFrees(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	e.Loop("fire@3,4", "amb.campfire", 0, 0)
	if len(b.streams) != 0 {
		t.Fatal("vol 0 started a loop")
	}
	e.Loop("fire@3,4", "amb.campfire", 0.5, -1)
	if len(b.streams) != 1 {
		t.Fatalf("loop voices %d", len(b.streams))
	}
	p := b.streams[0]
	lr, ok := p.r.(*loopReader)
	if !ok || lr.pan.get() != -1 {
		t.Fatalf("WAV loop reader %T pan", p.r)
	}
	for range tps { // 1 s > 0.8 s fade
		e.Loop("fire@3,4", "amb.campfire", 0.5, 0.25)
		e.Update()
	}
	if want := 0.5 * 0.7; !near(p.vol, want) || len(b.streams) != 1 {
		t.Fatalf("loop vol %v, want %v (voices %d)", p.vol, want, len(b.streams))
	}
	if lr.pan.get() != 0.25 {
		t.Fatalf("pan %v", lr.pan.get())
	}
	e.Loop("fire@3,4", "amb.campfire", 0, 0)
	for range tps {
		e.Update()
	}
	if !p.closed || len(e.loops) != 0 {
		t.Fatalf("faded loop closed %v, loops %d", p.closed, len(e.loops))
	}
}

func TestOGGLoopAndSwap(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	e.Loop("bed", "amb.day", 1, 0.5)
	if len(b.streams) != 1 {
		t.Fatal("no OGG loop")
	}
	if _, ok := b.streams[0].r.(*loopReader); ok {
		t.Fatal("OGG loop used the WAV reader")
	}
	for range tps {
		e.Update()
	}
	e.Loop("bed", "amb.campfire", 1, 0) // same key, new id: old fades out
	for range tps {
		e.Update()
	}
	if !b.streams[0].closed || b.streams[1].closed || len(e.loops) != 1 {
		t.Fatalf("swap: old closed %v new closed %v loops %d", b.streams[0].closed, b.streams[1].closed, len(e.loops))
	}
}

func TestMusicCrossfade(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	e.Music("music.calm", 0.8)
	for range 3 * tps {
		e.Music("music.calm", 0.8)
		e.Update()
	}
	old := b.streams[0]
	if !near(old.vol, 0.8) {
		t.Fatalf("music vol %v", old.vol)
	}
	e.Music("music.suite", 0.6)
	nu := b.streams[1]
	e.Update()
	if old.closed || old.vol >= 0.8 || nu.vol <= 0 {
		t.Fatalf("mid crossfade: old %v (closed %v), new %v", old.vol, old.closed, nu.vol)
	}
	for range 3 * tps {
		e.Update()
	}
	if !old.closed || len(e.fading) != 0 {
		t.Fatalf("old track closed %v, fading %d", old.closed, len(e.fading))
	}
	if !near(nu.vol, 0.6) || nu.closed {
		t.Fatalf("new track vol %v closed %v", nu.vol, nu.closed)
	}
	e.Music("", 0)
	for range 3 * tps {
		e.Update()
	}
	if !nu.closed || e.music != nil {
		t.Fatal(`Music("") left the track playing`)
	}
}

func TestMusicSilenceThenNextFile(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	e.Music("music.suite", 1)
	e.Update()
	first := b.streams[0]
	first.playing = false // the file ended
	e.Update()
	if !first.closed {
		t.Fatal("finished file not closed")
	}
	wait := e.music.wait
	if wait < gapMin*tps || wait > gapMax*tps {
		t.Fatalf("silence %d frames", wait)
	}
	for range wait - 1 {
		e.Update()
	}
	if len(b.streams) != 1 {
		t.Fatal("next file started before the silence ended")
	}
	e.Update()
	if len(b.streams) != 2 || b.streams[1].plays != 1 {
		t.Fatalf("second play: streams %d", len(b.streams))
	}
	// files play in turn: s1, s2, s1
	if e.music.next != 0 {
		t.Fatalf("next file index %d", e.music.next)
	}
}

func TestLoopingMusicNeverWaits(t *testing.T) {
	t.Parallel()
	e, b := testEngine(t)
	e.Music("music.calm", 1)
	for range 10 * tps {
		e.Update()
	}
	if len(b.streams) != 1 || b.streams[0].closed {
		t.Fatal("looping music restarted or stopped")
	}
}
