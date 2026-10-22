package ebitenaudio

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"rimworld-go/internal/platform/config"
	"rimworld-go/pkg/audiomanifest"
)

// stereoStub is the decoded form of a fake OGG: 16-bit LE stereo frames.
func stereoStub(frames int, l, r int16) []byte {
	b := make([]byte, 4*frames)
	for i := range frames {
		binary.LittleEndian.PutUint16(b[4*i:], uint16(l))
		binary.LittleEndian.PutUint16(b[4*i+2:], uint16(r))
	}
	return b
}

// oggEngine has an OGG one-shot "animal.test.call" with n variants, an OGG
// loop and a non-looping OGG music track. decode is counted and returns
// the same 100-frame stereo stub for every file (L 1000, R 3000).
func oggEngine(t *testing.T, variants int) (*Engine, *fakeBackend, *int) {
	t.Helper()
	files := make([]string, variants)
	oggs := make([][]byte, variants)
	for i := range files {
		files[i] = "animals/v.ogg"
		oggs[i] = []byte{byte(i)}
	}
	m := audiomanifest.Manifest{Version: 1, Sounds: []audiomanifest.Entry{
		entry("animal.test.call", audiomanifest.BusEffects, 1, false, files...),
		entry("amb.bed", audiomanifest.BusAmbience, 1, true, "bed.ogg"),
		entry("music.suite", audiomanifest.BusMusic, 1, false, "s1.ogg"),
	}}
	b := &fakeBackend{}
	e := newEngine(m, nil, map[string][][]byte{
		"animal.test.call": oggs,
		"amb.bed":          {{9}},
		"music.suite":      {{8}},
	}, b)
	decodes := 0
	e.decode = func([]byte) (io.ReadSeeker, int64, error) {
		decodes++
		d := stereoStub(100, 1000, 3000)
		return bytes.NewReader(d), int64(len(d)), nil
	}
	e.focused = func() bool { return true }
	e.SetVolumes(config.Audio{Master: 100, Music: 100, Effects: 100, Ambience: 100, UI: 100})
	return e, b, &decodes
}

func TestOggOneShotPanned(t *testing.T) {
	t.Parallel()
	e, b, _ := oggEngine(t, 1)
	e.Play("animal.test.call", 1, -1)
	e.Play("animal.test.call", 1, 1)
	if len(b.bytes) != 2 || len(b.streams) != 0 {
		t.Fatalf("one-shots: %d byte voices, %d streams", len(b.bytes), len(b.streams))
	}
	if bytes.Equal(b.bytes[0].data, b.bytes[1].data) {
		t.Fatal("pan -1 and +1 gave the same stereo bytes")
	}
	// Mono is the L/R average (2000); pan -1 silences the right channel.
	l, r := frames(b.bytes[0].data)
	if len(l) != 100 || l[0] != 2000 || r[0] != 0 {
		t.Fatalf("pan -1: %d frames, L %d R %d", len(l), l[0], r[0])
	}
	l, r = frames(b.bytes[1].data)
	if l[0] != 0 || r[0] != 2000 {
		t.Fatalf("pan +1: L %d R %d", l[0], r[0])
	}
}

func TestOggDecodeCached(t *testing.T) {
	t.Parallel()
	e, b, decodes := oggEngine(t, 1)
	for range 3 {
		e.Play("animal.test.call", 1, 0)
	}
	if *decodes != 1 || len(b.bytes) != 3 {
		t.Fatalf("decodes %d, voices %d; want 1 and 3", *decodes, len(b.bytes))
	}
}

func TestOggCacheEvictsLRU(t *testing.T) {
	t.Parallel()
	e, _, decodes := oggEngine(t, 3)
	// Each decoded variant is 100 mono samples = 200 bytes; room for two.
	e.oggCache.limit = 2 * 200
	play := func(v int) {
		p, _, err := e.openShot("animal.test.call", v, 0)
		if err != nil || p == nil {
			t.Fatalf("variant %d: %v", v, err)
		}
	}
	play(0)
	play(1)
	play(0) // 0 is now the most recent, 1 the least
	play(2) // evicts 1
	if *decodes != 3 {
		t.Fatalf("decodes %d, want 3", *decodes)
	}
	play(0) // still cached
	if *decodes != 3 {
		t.Fatalf("variant 0 decoded again (%d)", *decodes)
	}
	play(1) // was evicted
	if *decodes != 4 {
		t.Fatalf("variant 1 should decode again: %d", *decodes)
	}
	if e.oggCache.bytes > e.oggCache.limit {
		t.Fatalf("cache holds %d bytes over its %d limit", e.oggCache.bytes, e.oggCache.limit)
	}
}

func TestOggCacheDefaultLimit(t *testing.T) {
	t.Parallel()
	e, _, _ := oggEngine(t, 1)
	if e.oggCache.limit != 24<<20 {
		t.Fatalf("cache limit %d, want 24 MiB", e.oggCache.limit)
	}
}

func TestOggTooBigForCacheStillPlays(t *testing.T) {
	t.Parallel()
	e, b, decodes := oggEngine(t, 1)
	e.oggCache.limit = 10
	e.Play("animal.test.call", 1, 0)
	e.Play("animal.test.call", 1, 0)
	if len(b.bytes) != 2 || *decodes != 2 || e.oggCache.bytes != 0 {
		t.Fatalf("voices %d, decodes %d, cached %d", len(b.bytes), *decodes, e.oggCache.bytes)
	}
}

func TestOggDecodeErrorIsWarnedNotCached(t *testing.T) {
	t.Parallel()
	e, b, _ := oggEngine(t, 1)
	e.decode = func([]byte) (io.ReadSeeker, int64, error) { return nil, 0, io.ErrUnexpectedEOF }
	e.Play("animal.test.call", 1, 0)
	if len(b.bytes) != 0 || len(e.shots) != 0 || e.oggCache.bytes != 0 {
		t.Fatal("a failed decode made a voice or a cache entry")
	}
}

func TestLoopsAndMusicUnchanged(t *testing.T) {
	t.Parallel()
	e, b, decodes := oggEngine(t, 1)
	e.Loop("bed", "amb.bed", 1, 0.5)
	e.Music("music.suite", 1)
	if len(b.streams) != 2 || len(b.bytes) != 0 {
		t.Fatalf("loop+music: %d streams, %d byte voices", len(b.streams), len(b.bytes))
	}
	if _, ok := b.streams[0].r.(*loopReader); ok {
		t.Fatal("OGG loop used the WAV reader")
	}
	if *decodes != 2 || e.oggCache.bytes != 0 || e.oggCache.ll.Len() != 0 {
		t.Fatalf("loops/music touched the cache: decodes %d, cached %d", *decodes, e.oggCache.bytes)
	}
}
