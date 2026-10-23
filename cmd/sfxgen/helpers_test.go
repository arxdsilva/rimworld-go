package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeKey is a stand-in; the tests assert it never reaches any output.
const fakeKey = "sk-fake-TEST-KEY-0123456789abcdef"

// fakeAPI is an httptest ElevenLabs: scripted failures first, then MP3s.
type fakeAPI struct {
	*httptest.Server
	mu     sync.Mutex
	gens   []map[string]any // generation request bodies
	subs   int              // subscription reads
	keys   []string         // xi-api-key seen
	script []int            // statuses for the first generation calls
	used   int
	limit  int
	perGen int // credits the counter moves per generation
	// lag, when set, holds the counter still until every lag-th
	// generation, then shows everything at once (the real API lags).
	lag     int
	shown   int
	echoKey bool
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{limit: 10000, perGen: 20}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.keys = append(f.keys, r.Header.Get("xi-api-key"))
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/user/subscription":
			f.subs++
			shown := f.used
			if f.lag > 0 {
				shown = f.shown
			}
			json.NewEncoder(w).Encode(map[string]int{"character_count": shown, "character_limit": f.limit})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sound-generation":
			if r.URL.Query().Get("output_format") != "mp3_44100_128" {
				http.Error(w, "bad format", http.StatusBadRequest)
				return
			}
			var body map[string]any
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &body)
			f.gens = append(f.gens, body)
			if len(f.script) > 0 {
				code := f.script[0]
				f.script = f.script[1:]
				if code != http.StatusOK {
					msg := `{"detail":"scripted"}`
					if f.echoKey { // a hostile server repeating the header
						msg = `{"detail":"bad key ` + r.Header.Get("xi-api-key") + `"}`
					}
					http.Error(w, msg, code)
					return
				}
			}
			f.used += f.perGen
			if f.lag > 0 && len(f.gens)%f.lag == 0 {
				f.shown = f.used
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			fmt.Fprintf(w, "FAKE-MP3:%v", body["text"])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeAPI) generations() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.gens)
}

// env is one test workspace.
type env struct {
	t                                 *testing.T
	dir, prompts, ledger, raw, assets string
	key                               string
	api                               *fakeAPI
	lufs                              func(string) (float64, error)
}

type row struct {
	id  string
	sec float64
}

func newEnv(t *testing.T, rows ...row) *env {
	t.Helper()
	dir := t.TempDir()
	e := &env{t: t, dir: dir, prompts: filepath.Join(dir, "prompts.json"),
		ledger: filepath.Join(dir, "elevenlabs", "ledger.json"), raw: filepath.Join(dir, "elevenlabs", "raw"),
		assets: filepath.Join(dir, "assets"), key: fakeKey}
	var sounds []map[string]any
	for _, r := range rows {
		sounds = append(sounds, map[string]any{"id": r.id, "seconds": r.sec, "prompt": "prompt for " + r.id})
	}
	p := map[string]any{"version": 1, "defaults": map[string]any{"prompt_influence": 0.55, "suffix": ", dry"}, "sounds": sounds}
	b, _ := json.Marshal(p)
	if err := os.WriteFile(e.prompts, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *env) writeLedger(takes ...Take) {
	e.t.Helper()
	l := &Ledger{Note: "test ledger", Takes: takes, Totals: json.RawMessage(`{"credits_used": 600}`)}
	if err := l.save(e.ledger); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) readLedger() *Ledger {
	e.t.Helper()
	l, err := loadLedger(e.ledger)
	if err != nil {
		e.t.Fatal(err)
	}
	return l
}

// cli runs sfxgen against the env and returns stdout and stderr.
func (e *env) cli(args ...string) (string, string, error) {
	e.t.Helper()
	base := []string{"-prompts", e.prompts, "-ledger", e.ledger, "-out", e.raw, "-assets", e.assets, "-backoff", "1ms"}
	if e.api != nil {
		base = append(base, "-base-url", e.api.URL)
	}
	var out, errb bytes.Buffer
	getenv := func(k string) string {
		if k == keyEnv {
			return e.key
		}
		return ""
	}
	a, err := newApp(append(base, args...), getenv, &out, &errb)
	if err == nil && a != nil {
		if e.lufs != nil {
			a.lufs = e.lufs
		}
		err = a.dispatch()
	}
	return out.String(), errb.String(), err
}

func contains(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			t.Fatalf("missing %q in:\n%s", n, haystack)
		}
	}
}
