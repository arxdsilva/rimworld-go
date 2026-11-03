package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"rimworld-go/pkg/audiomanifest"
)

// mossEnv is an env on the MOSS backend with stand-in python, runner and
// model files (the preflight only checks they exist).
func mossEnv(t *testing.T, rows ...row) (*env, []string) {
	t.Helper()
	e := newEnv(t, rows...)
	e.backend = backendMoss
	e.key = "" // MOSS needs no key
	py, runner, model := filepath.Join(e.dir, "venv/bin/python"), filepath.Join(e.dir, "generate.py"), filepath.Join(e.dir, "model")
	for _, f := range []string{py, runner, filepath.Join(model, "model_index.json")} {
		os.MkdirAll(filepath.Dir(f), 0o755)
		if err := os.WriteFile(f, []byte("stand-in"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return e, []string{"-moss-python", py, "-moss-runner", runner, "-moss-model", model}
}

func TestBackendSelection(t *testing.T) {
	t.Parallel()
	env := map[string]string{"HOME": "/home/u"}
	getenv := func(k string) string { return env[k] }
	a, err := newApp(nil, getenv, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	m := a.cfg.moss
	if a.cfg.backend != backendMoss || a.cfg.out != "assets/source/audio/moss/raw" {
		t.Fatalf("default backend %q out %q, want moss and its raw dir", a.cfg.backend, a.cfg.out)
	}
	if m.python != "/home/u/.cache/rimworld-go/moss/venv/bin/python" || m.model != "/home/u/.cache/rimworld-go/moss/MOSS-SoundEffect-v2.0" ||
		m.runner != "cmd/sfxgen/moss/generate.py" || m.steps != 100 || m.cfgScale != 4 || m.device != "auto" || m.minFreeGB != 24 {
		t.Fatalf("moss defaults = %+v", m)
	}
	env["XDG_CACHE_HOME"] = "/cache"
	if a, _ = newApp(nil, getenv, io.Discard, io.Discard); !strings.HasPrefix(a.cfg.moss.python, "/cache/rimworld-go/moss/") {
		t.Fatalf("XDG_CACHE_HOME ignored: %s", a.cfg.moss.python)
	}
	env[mossPythonEnv], env[mossModelEnv] = "/opt/py", "/opt/model"
	if a, _ = newApp(nil, getenv, io.Discard, io.Discard); a.cfg.moss.python != "/opt/py" || a.cfg.moss.model != "/opt/model" {
		t.Fatalf("env paths ignored: %+v", a.cfg.moss)
	}
	if a, _ = newApp([]string{"-moss-python", "/flag/py"}, getenv, io.Discard, io.Discard); a.cfg.moss.python != "/flag/py" {
		t.Fatal("the flag should beat the environment")
	}
	a, err = newApp([]string{"-backend", "elevenlabs"}, getenv, io.Discard, io.Discard)
	if err != nil || a.cfg.backend != backendElevenLabs || a.cfg.out != "assets/source/audio/elevenlabs/raw" {
		t.Fatalf("elevenlabs: %v %+v", err, a)
	}
	if a, _ = newApp([]string{"-out", "x/raw"}, getenv, io.Discard, io.Discard); a.cfg.out != "x/raw" {
		t.Fatal("-out ignored")
	}
	if _, err := newApp([]string{"-backend", "suno"}, getenv, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "moss") {
		t.Fatalf("unknown backend: %v", err)
	}
}

func TestMossCommandLine(t *testing.T) {
	t.Parallel()
	a, err := newApp([]string{"-moss-python", "/v/bin/python", "-moss-model", "/m", "-moss-runner", "r.py",
		"-moss-steps", "50", "-moss-cfg", "3.5", "-moss-device", "mps"}, func(string) string { return "" }, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got := a.mossCommand("a robot whirs, dry", 0.75, 42, "out/x.wav")
	want := []string{"/v/bin/python", "-I", "r.py", "--model", "/m", "--prompt", "a robot whirs, dry", "--seconds", "0.8",
		"--seed", "42", "--steps", "50", "--cfg", "3.5", "--device", "mps", "--out", "out/x.wav"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("command\n got %q\nwant %q", got, want)
	}
	// seeds: stable per id and take, distinct between takes, -moss-seed wins
	first, again, take2 := a.mossSeed("animal.cow.call", 1), a.mossSeed("animal.cow.call", 1), a.mossSeed("animal.cow.call", 2)
	if first != again || first == take2 {
		t.Fatal("derived seeds must be stable and differ by take")
	}
	if s := first; s <= 0 {
		t.Fatalf("seed %d", s)
	}
	a.cfg.moss.seed = 7
	if a.mossSeed("animal.cow.call", 3) != 7 {
		t.Fatal("-moss-seed ignored")
	}
}

func TestMossDryRunRunsNothing(t *testing.T) {
	t.Parallel()
	e, flags := mossEnv(t, row{"animal.cow.call", 1.5}, row{"animal.cow.hurt", 0.6})
	out, _, err := e.cli(append(flags, "-dry-run")...)
	if err != nil {
		t.Fatal(err)
	}
	contains(t, out, "animal.cow.call", "animal.cow.hurt", "total: 2 sounds", "no credits", "command: ", " -I ", "--prompt prompt for animal.cow.call, dry", "--seconds 1.5")
	if strings.Contains(out, "warning") {
		t.Fatalf("preflight complained: %s", out)
	}
	// a missing model is a warning in a dry run and an error in a real one
	out, _, err = e.cli(append(flags, "-dry-run", "-moss-model", filepath.Join(e.dir, "nowhere"))...)
	if err != nil || !strings.Contains(out, "warning: MOSS") {
		t.Fatalf("dry run without a model: %v\n%s", err, out)
	}
	if _, _, err := e.cli(append(flags, "-moss-model", filepath.Join(e.dir, "nowhere"))...); err == nil || !strings.Contains(err.Error(), "MOSS model not found") {
		t.Fatalf("real run without a model: %v", err)
	}
}

// fakeRunner stands in for the MOSS runner: it records each command and
// writes a tone WAV where --out points.
type fakeRunner struct {
	t     *testing.T
	mu    sync.Mutex
	calls [][]string
	fail  error
}

func (f *fakeRunner) run(_ context.Context, argv []string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, argv)
	if f.fail != nil {
		return []byte("Traceback: boom"), f.fail
	}
	for i, a := range argv {
		if a == "--out" {
			writeRaw(f.t, argv[i+1])
		}
	}
	return []byte("loading...\n{\"device\": \"mps\", \"gen_seconds\": 201.5, \"peak\": 0.9}\n"), nil
}

func TestMossGenerateLogsProcessesAndApproves(t *testing.T) {
	needTools(t)
	t.Parallel()
	e, flags := mossEnv(t, row{"animal.cow.call", 1.5}, row{"animal.cow.die", 1.8}, row{"animal.pig.call", 1})
	f := &fakeRunner{t: t}
	e.runMoss = f.run
	out, _, err := e.cli(append(flags, "-only", "animal.cow.*")...)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 {
		t.Fatalf("%d runner calls, want 2", len(f.calls))
	}
	contains(t, out, "animal.cow.call take 1", "made in 202 s on mps", "animal.cow.call processed:", "variant b", "no credits")
	l := e.readLedger()
	if len(l.Takes) != 2 {
		t.Fatalf("ledger = %+v", l.Takes)
	}
	tk := l.Takes[0]
	if tk.ID != "animal.cow.call" || tk.Backend != backendMoss || tk.Tool != mossTool || tk.Credits != 0 || tk.Seed != (&app{}).mossSeed("animal.cow.call", 1) ||
		tk.File != "raw/animal.cow.call_t1.wav" || tk.Prompt != "prompt for animal.cow.call, dry" || tk.Seconds != 1.5 || tk.state() != statePending {
		t.Fatalf("take = %+v", tk)
	}
	dir := filepath.Join(e.assets, "generated/audio/sfx/animals")
	for _, name := range []string{"cow_call.ogg", "cow_call_b.ogg", "cow_die.ogg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s not processed: %v", name, err)
		}
	}
	// a pending take is not made again
	out, _, _ = e.cli(append(flags, "-only", "animal.cow.call")...)
	contains(t, out, "skip animal.cow.call: take waiting for review")
	// the review page names the backend; approve lists the MOSS provenance
	e.lufs = func(string) (float64, error) { return -20, nil }
	page := filepath.Join(e.dir, "review.html")
	if _, _, err := e.cli("-review", "-o", page); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(page)
	contains(t, string(b), "<td>moss</td>")
	mpath := filepath.Join(e.assets, filepath.FromSlash(audiomanifest.Rel))
	os.MkdirAll(filepath.Dir(mpath), 0o755)
	writeManifestFile(t, mpath, audiomanifest.Manifest{Sounds: []audiomanifest.Entry{{ID: "animal.cow.call", Bus: "effects", Volume: 0.7,
		Files: []string{"sfx/animals/old.ogg"}, Source: audiomanifest.SourceElevenLabs, License: elevenLabsLicense}}}) // a generated entry is replaced
	if _, _, err := e.cli("-approve", "animal.cow.*", "-reviewer", "tester"); err != nil {
		t.Fatal(err)
	}
	c, _ := readManifestFile(t, mpath).Lookup("animal.cow.call")
	if c.Source != audiomanifest.SourceMOSS || c.Tool != mossTool || c.License != mossLicense || c.Credits != 0 ||
		c.Seed != uint64(tk.Seed) || c.Review != "tester" || len(c.Files) != 2 {
		t.Fatalf("manifest entry = %+v", c)
	}
	if e.readLedger().Takes[0].Approved != "tester" {
		t.Fatal("ledger not approved")
	}
}

func TestMossRejectedIsMadeAgainAndFailuresStop(t *testing.T) {
	t.Parallel()
	e, flags := mossEnv(t, row{"animal.cow.call", 1}, row{"animal.cow.hurt", 0.6})
	e.writeLedger(
		Take{ID: "animal.cow.call", Take: 1, Backend: backendMoss, Approved: "rejected"},
		Take{ID: "animal.cow.call", Take: 2, Backend: backendMoss, Approved: "rejected"},
	)
	f := &fakeRunner{t: t, fail: errors.New("exit status 1")}
	e.runMoss = f.run
	_, _, err := e.cli(append(flags, "-retry")...)
	if err == nil || !strings.Contains(err.Error(), "animal.cow.call") {
		t.Fatalf("a failed take should stop the run: %v", err)
	}
	// -retry made only the twice-rejected sound (no cap when free), as take 3
	if len(f.calls) != 1 || !strings.Contains(strings.Join(f.calls[0], " "), "animal.cow.call_t3.wav") {
		t.Fatalf("calls = %q", f.calls)
	}
	if n := len(e.readLedger().Takes); n != 2 {
		t.Fatalf("a failed take was logged (%d takes)", n)
	}
	todo, skips := decide([]Sound{{ID: "animal.cow.call"}}, e.readLedger(), false, false)
	if len(todo) != 0 || len(skips) != 1 || !strings.Contains(skips[0].why, "rejected twice") {
		t.Fatal("the paid backend keeps its retry cap")
	}
}

func TestMossWaitsForMemory(t *testing.T) {
	t.Parallel()
	e, flags := mossEnv(t, row{"animal.cow.call", 1})
	readings := []float64{10, 12, 30}
	var mu sync.Mutex
	e.freeGB = func() (float64, bool) {
		mu.Lock()
		defer mu.Unlock()
		v := readings[0]
		if len(readings) > 1 {
			readings = readings[1:]
		}
		return v, true
	}
	ran := 0
	e.runMoss = func(_ context.Context, argv []string) ([]byte, error) {
		ran++
		return nil, errors.New("stop here")
	}
	out, _, _ := e.cli(append(flags, "-moss-poll", "1s")...)
	if strings.Count(out, "wait: ") != 2 || ran != 1 {
		t.Fatalf("want two waits then the take (ran %d):\n%s", ran, out)
	}
	// memory that never frees gives up without running anything
	e2, flags2 := mossEnv(t, row{"animal.cow.call", 1})
	e2.freeGB = func() (float64, bool) { return 8, true }
	_, _, err := e2.cli(append(flags2, "-moss-poll", "1m", "-moss-max-wait", "3m")...)
	if err == nil || !strings.Contains(err.Error(), "-moss-min-free-gb") {
		t.Fatalf("want a give-up error: %v", err)
	}
}

func TestParseVMStatAndReport(t *testing.T) {
	t.Parallel()
	out := "Mach Virtual Memory Statistics: (page size of 16384 bytes)\nPages free:                               693492.\n" +
		"Pages active:                            1492876.\nPages inactive:                          1448376.\n"
	gb, err := parseVMStat(out)
	if err != nil || gb < 35.0 || gb > 35.2 {
		t.Fatalf("parseVMStat = %v, %v; want about 35.09 GB", gb, err)
	}
	if _, err := parseVMStat("nothing"); err == nil {
		t.Fatal("garbage parsed")
	}
	r, ok := lastReport([]byte("noise {not json\n{\"device\":\"cpu\",\"gen_seconds\":1}\n{\"device\":\"mps\",\"gen_seconds\":2.5}\ntrailing\n"))
	if !ok || r.Device != "mps" || r.GenSeconds != 2.5 {
		t.Fatalf("lastReport = %+v %v", r, ok)
	}
}

func TestLedgerPathAndRawPath(t *testing.T) {
	t.Parallel()
	a := &app{cfg: config{ledger: "assets/source/audio/elevenlabs/ledger.json"}}
	for raw, want := range map[string]string{
		"assets/source/audio/elevenlabs/raw/x.mp3": "raw/x.mp3",
		"assets/source/audio/moss/raw/x.wav":       "../moss/raw/x.wav",
		"/abs/x.wav":                               "/abs/x.wav",
	} {
		got := a.ledgerPath(raw)
		if got != want {
			t.Errorf("ledgerPath(%s) = %s, want %s", raw, got, want)
		}
		if back := a.rawPath(Take{File: got}); filepath.Clean(back) != filepath.Clean(raw) {
			t.Errorf("rawPath(%s) = %s, want %s", got, back, raw)
		}
	}
}

func TestMossProcessingFailureKeepsGoing(t *testing.T) {
	t.Parallel()
	e, flags := mossEnv(t, row{"animal.cow.call", 1}, row{"animal.cow.hurt", 0.6})
	f := &fakeRunner{t: t}
	e.runMoss = f.run
	e.ffmpeg = filepath.Join(e.dir, "no-such-ffmpeg")
	out, _, err := e.cli(flags...)
	if err == nil || !strings.Contains(err.Error(), "animal.cow.call,animal.cow.hurt") {
		t.Fatalf("want both failures reported: %v", err)
	}
	if len(f.calls) != 2 || len(e.readLedger().Takes) != 2 {
		t.Fatalf("a processing failure stopped the run: %d calls\n%s", len(f.calls), out)
	}
	contains(t, out, "processing failed", "-reject animal.cow.call")
}
