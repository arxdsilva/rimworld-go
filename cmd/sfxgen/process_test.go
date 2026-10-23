package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeTone writes a mono 44.1 kHz WAV: 0.25 s of silence, a decaying
// two-harmonic tone of 0.6 s, 0.25 s of silence.
func writeTone(t *testing.T, path string) { writeToneOf(t, path, 0.6) }

// writeToneOf is writeTone with a tone of tone seconds.
func writeToneOf(t *testing.T, path string, tone float64) {
	t.Helper()
	const sr = 44100
	toneN := int(tone * sr)
	n := sr/2 + toneN
	pcm := make([]int16, n)
	for i := sr / 4; i < sr/4+toneN; i++ {
		x := float64(i-sr/4) / sr
		v := 0.3 * math.Exp(-3*x) * (math.Sin(2*math.Pi*300*x) + 0.5*math.Sin(2*math.Pi*600*x))
		pcm[i] = int16(v * 32767)
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+2*n))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(sr), uint32(sr * 2), uint16(2), uint16(16)} {
		binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(2*n))
	binary.Write(&b, binary.LittleEndian, pcm)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeRaw makes a raw take: an MP3 when ffmpeg can encode one, else the
// WAV (ffmpeg probes content, not the extension).
func writeRaw(t *testing.T, path string) { writeRawOf(t, path, 0.6) }

// writeRawOf is writeRaw with a tone of tone seconds.
func writeRawOf(t *testing.T, path string, tone float64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	wav := path + ".wav"
	writeToneOf(t, wav, tone)
	if err := exec.Command("ffmpeg", "-hide_banner", "-nostdin", "-y", "-i", wav, "-c:a", "libmp3lame", "-b:a", "128k", "-f", "mp3", path).Run(); err != nil {
		b, _ := os.ReadFile(wav)
		os.WriteFile(path, b, 0o644)
	}
	os.Remove(wav)
}

func needTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "oggenc"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
}

// decodedSeconds decodes an Ogg with ffmpeg and returns its length and
// stream description.
func decodedSeconds(t *testing.T, ogg string) (float64, string) {
	t.Helper()
	wav := filepath.Join(t.TempDir(), "d.wav")
	out, err := exec.Command("ffmpeg", "-hide_banner", "-nostdin", "-y", "-i", ogg, "-c:a", "pcm_s16le", wav).CombinedOutput()
	if err != nil {
		t.Fatalf("%s does not decode: %v\n%s", ogg, err, out)
	}
	d, err := wavSeconds(wav)
	if err != nil {
		t.Fatal(err)
	}
	return d, string(out)
}

func TestProcessMakesOggAndVariant(t *testing.T) {
	needTools(t)
	t.Parallel()
	e := newEnv(t)
	writeRaw(t, filepath.Join(e.raw, "animal.cow.call_t1.mp3"))
	writeRaw(t, filepath.Join(e.raw, "animal.cow.die_t1.mp3"))
	writeRaw(t, filepath.Join(e.raw, "animal.cow.burst_t1.mp3"))
	rel := func(id string) string { return "raw/" + id + "_t1.mp3" }
	e.writeLedger(
		Take{ID: "animal.cow.call", Take: 1, File: rel("animal.cow.call"), Approved: "arxdsilva"},
		Take{ID: "animal.cow.die", Take: 1, File: rel("animal.cow.die")}, // pending
	)
	dir := filepath.Join(e.assets, "generated/audio/sfx/animals")

	// without -only: approved takes only
	out, _, err := e.cli("-process")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, out, "animal.cow.call take 1", "variant b")
	for _, f := range []string{"cow_call.ogg", "cow_call_b.ogg"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s missing: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "cow_die.ogg")); err == nil {
		t.Fatal("a pending take was processed without -only")
	}
	a, desc := decodedSeconds(t, filepath.Join(dir, "cow_call.ogg"))
	contains(t, desc, "48000 Hz, mono")
	b, _ := decodedSeconds(t, filepath.Join(dir, "cow_call_b.ogg"))
	// the tone is 0.6 s inside 1.1 s: both silences are trimmed
	if a < 0.4 || a > 0.75 {
		t.Fatalf("A is %.3f s, want about 0.6 (silence trimmed)", a)
	}
	// pitched +-4%: shorter or longer by about the rate
	if r := a / b; math.Abs(r-variantRate("animal.cow.call")) > 0.03 {
		t.Fatalf("B/A length ratio %.3f, want about %.2f", r, variantRate("animal.cow.call"))
	}
	// loudness lands on the call target (-20 LUFS)
	ap := &app{ffmpeg: "ffmpeg"}
	if l, err := ap.measureLUFS(filepath.Join(dir, "cow_call.ogg")); err != nil || math.Abs(l+20) > 1.5 {
		t.Fatalf("A is %.1f LUFS (%v), want about -20", l, err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, "cow_call.ogg"))
	if !bytes.HasPrefix(first, []byte("OggS")) || !bytes.Contains(first, []byte("vorbis")) {
		t.Fatal("not an Ogg Vorbis file")
	}

	// -only processes a pending take; die has no variant
	if _, _, err := e.cli("-process", "-only", "animal.cow.die"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cow_die.ogg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cow_die_b.ogg")); err == nil {
		t.Fatal("die got a variant")
	}

	// deterministic: a second run is byte-identical
	if _, _, err := e.cli("-process"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "cow_call.ogg"))
	if !bytes.Equal(first, second) {
		t.Fatal("processing is not deterministic")
	}

	// the burst loudness target is the loudest
	if _, ok := events["burst"]; !ok || events["burst"].lufs != -16 {
		t.Fatal("burst target")
	}
}

// Loudness is measured over 400 ms blocks: a hurt yelp trimmed to 0.3 s
// must still be measured and normalised, and keep its length.
func TestProcessShortClip(t *testing.T) {
	needTools(t)
	t.Parallel()
	e := newEnv(t)
	writeRawOf(t, filepath.Join(e.raw, "animal.cow.hurt_t1.mp3"), 0.3)
	e.writeLedger(Take{ID: "animal.cow.hurt", Take: 1, File: "raw/animal.cow.hurt_t1.mp3"})
	if _, _, err := e.cli("-process", "-only", "animal.cow.hurt"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(e.assets, "generated/audio/sfx/animals")
	for _, f := range []string{"cow_hurt.ogg", "cow_hurt_b.ogg"} {
		d, _ := decodedSeconds(t, filepath.Join(dir, f))
		if d < 0.2 || d > 0.36 {
			t.Fatalf("%s is %.3f s, want the 0.3 s clip (no padding left)", f, d)
		}
	}
}

func TestProcessErrors(t *testing.T) {
	needTools(t)
	t.Parallel()
	e := newEnv(t)
	// nothing approved: says so
	e.writeLedger(Take{ID: "animal.cow.call", Take: 1, File: "raw/x.mp3"})
	out, _, err := e.cli("-process")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, out, "nothing to do")
	// a silent clip is refused with a clear message
	silent := filepath.Join(e.raw, "s.wav")
	os.MkdirAll(e.raw, 0o755)
	if err := exec.Command("ffmpeg", "-hide_banner", "-nostdin", "-y", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono", "-t", "0.5", silent).Run(); err != nil {
		t.Fatal(err)
	}
	e.writeLedger(Take{ID: "animal.cow.call", Take: 1, File: "raw/s.wav", Approved: "arxdsilva"})
	if _, _, err := e.cli("-process"); err == nil {
		t.Fatal("a silent clip was processed")
	}
	// a missing raw file
	e.writeLedger(Take{ID: "animal.cow.call", Take: 1, File: "raw/none.mp3", Approved: "arxdsilva"})
	if _, _, err := e.cli("-process"); err == nil || !strings.Contains(err.Error(), "animal.cow.call") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseLoudnormAndWAV(t *testing.T) {
	t.Parallel()
	good := "noise\n{\n\t\"input_i\" : \"-21.75\",\n\t\"input_tp\" : \"-18.06\",\n\t\"input_lra\" : \"0.00\",\n\t\"input_thresh\" : \"-31.75\",\n\t\"output_i\" : \"-20.05\",\n\t\"target_offset\" : \"0.05\"\n}\n"
	m, err := parseLoudnorm(good)
	if err != nil || m.InputI != "-21.75" || m.OutputI != "-20.05" {
		t.Fatalf("%+v, %v", m, err)
	}
	for _, bad := range []string{"", "{ \"input_i\": \"-inf\"", strings.Replace(good, "-21.75", "-inf", 1), "{not json}"} {
		if _, err := parseLoudnorm(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	p := filepath.Join(t.TempDir(), "t.wav")
	writeTone(t, p)
	if d, err := wavSeconds(p); err != nil || math.Abs(d-1.1) > 0.001 {
		t.Fatalf("wavSeconds = %v, %v", d, err)
	}
	os.WriteFile(p, []byte("nope"), 0o644)
	if _, err := wavSeconds(p); err == nil {
		t.Fatal("non-WAV accepted")
	}
	if _, err := wavSeconds(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestHelpersDeterministic(t *testing.T) {
	t.Parallel()
	if estimate(0.6, 10) != 6 || estimate(1.8, 10) != 18 || estimate(2, 10) != 20 {
		t.Fatal("estimate")
	}
	if stemOf("animal.timber_wolf.call") != "timber_wolf_call" || stemOf("animal.burst") != "burst" || eventOf("animal.burst") != "burst" {
		t.Fatal("stem/event")
	}
	if s := serialOf("a", "a"); s != serialOf("a", "a") || s == serialOf("a", "b") {
		t.Fatal("serial")
	}
	if r := variantRate("animal.cow.call"); r != 1.04 && r != 0.96 {
		t.Fatal(r)
	}
}
