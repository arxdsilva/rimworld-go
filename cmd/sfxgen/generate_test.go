package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func threeSounds() []row {
	return []row{{"animal.cow.call", 2}, {"animal.cow.hurt", 0.6}, {"animal.cow.die", 1.8}}
}

func TestDryRunEstimatesCredits(t *testing.T) {
	t.Parallel()
	e := newEnv(t, threeSounds()...)
	e.api = newFakeAPI(t)
	e.key = "" // a dry run needs no key
	e.writeLedger(Take{ID: "amb.day", Take: 1, Credits: 600, Approved: "arxdsilva"})
	out, _, err := e.cli("-dry-run")
	if err != nil {
		t.Fatal(err)
	}
	// 2.0 s -> 20, 0.6 s -> 6, 1.8 s -> 18
	contains(t, out, "animal.cow.call", "total: 3 sounds, 4.4 s, 44 credits",
		"10000 - 600 already spent in the ledger - 1500 reserve = 7900 usable; this plan leaves 7856")
	if e.api.generations() != 0 || e.api.subs != 0 {
		t.Fatal("a dry run touched the network")
	}
	if _, err := os.Stat(e.raw); err == nil {
		t.Fatal("a dry run wrote raw files")
	}
	out, _, err = e.cli("-dry-run", "-credit-rate", "20", "-only", "animal.cow.h*", "-max-credits", "5")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, out, "total: 1 sounds, 0.6 s, 12 credits", "-max-credits 5")
}

func TestDryRunUsesAPIBudgetWhenAsked(t *testing.T) {
	t.Parallel()
	e := newEnv(t, threeSounds()...)
	e.api = newFakeAPI(t)
	e.api.used = 9000
	out, _, err := e.cli("-dry-run", "-budget-from-api")
	if err != nil {
		t.Fatal(err)
	}
	// 10000 - 9000 - 1500 < 0: the API headroom is the smaller one
	contains(t, out, "= -500 usable")
	e.key = ""
	if _, _, err := e.cli("-dry-run", "-budget-from-api"); err == nil {
		t.Fatal("-budget-from-api without a key accepted")
	}
}

func TestStopsAtBudget(t *testing.T) {
	t.Parallel()
	rows := []row{{"animal.a.call", 2}, {"animal.b.call", 2}, {"animal.c.call", 2}, {"animal.d.call", 2}, {"animal.e.call", 2}}
	t.Run("ledger budget", func(t *testing.T) {
		e := newEnv(t, rows...)
		e.api = newFakeAPI(t)
		// 1550 - 1500 reserve = 50 usable: two 20-credit clips, not a third
		out, _, err := e.cli("-budget", "1550")
		if err != nil {
			t.Fatal(err)
		}
		if n := e.api.generations(); n != 2 {
			t.Fatalf("%d generations, want 2", n)
		}
		contains(t, out, "stop: animal.c.call needs about 20 credits but only 10 remain before the 1500 reserve")
		if got := len(e.readLedger().Takes); got != 2 {
			t.Fatalf("%d takes logged", got)
		}
	})
	t.Run("api budget and measured credits", func(t *testing.T) {
		e := newEnv(t, rows...)
		e.api = newFakeAPI(t)
		e.api.used, e.api.perGen = 8400, 25 // 100 left before the reserve; each take really costs 25
		out, _, err := e.cli("-budget-from-api")
		if err != nil {
			t.Fatal(err)
		}
		if n := e.api.generations(); n != 4 {
			t.Fatalf("%d generations, want 4", n)
		}
		contains(t, out, "stop: animal.e.call")
		for _, tk := range e.readLedger().Takes {
			if tk.Credits != 25 {
				t.Fatalf("recorded %d credits, the API measured 25", tk.Credits)
			}
		}
	})
	t.Run("nothing left", func(t *testing.T) {
		e := newEnv(t, rows...)
		e.api = newFakeAPI(t)
		e.api.used = 9000
		out, _, err := e.cli("-budget-from-api")
		if err != nil || e.api.generations() != 0 {
			t.Fatalf("err %v, %d generations", err, e.api.generations())
		}
		contains(t, out, "stop:")
	})
}

func TestStopsAtMaxCredits(t *testing.T) {
	t.Parallel()
	e := newEnv(t, row{"animal.a.call", 2}, row{"animal.b.call", 2}, row{"animal.c.call", 2})
	e.api = newFakeAPI(t)
	out, _, err := e.cli("-max-credits", "50")
	if err != nil {
		t.Fatal(err)
	}
	if n := e.api.generations(); n != 2 {
		t.Fatalf("%d generations, want 2 (40 of 50 credits)", n)
	}
	contains(t, out, "over -max-credits 50", "done: 2 takes, 40 credits")
}

func TestSkipsApprovedTakes(t *testing.T) {
	t.Parallel()
	e := newEnv(t, row{"animal.ok.call", 1}, row{"animal.waiting.call", 1}, row{"animal.once.call", 1},
		row{"animal.twice.call", 1}, row{"animal.new.call", 1})
	e.api = newFakeAPI(t)
	e.writeLedger(
		Take{ID: "animal.ok.call", Take: 1, Credits: 10, Approved: "arxdsilva"},
		Take{ID: "animal.waiting.call", Take: 1, Credits: 10},
		Take{ID: "animal.once.call", Take: 1, Credits: 10, Approved: "rejected"},
		Take{ID: "animal.twice.call", Take: 1, Credits: 10, Approved: "rejected"},
		Take{ID: "animal.twice.call", Take: 2, Credits: 10, Approved: "rejected"},
	)
	out, _, err := e.cli()
	if err != nil {
		t.Fatal(err)
	}
	if n := e.api.generations(); n != 1 {
		t.Fatalf("%d generations, want only animal.new.call", n)
	}
	contains(t, out, "skip animal.ok.call: already approved", "skip animal.waiting.call: take waiting for review",
		"skip animal.once.call: rejected once", "skip animal.twice.call: rejected twice")
	// the one allowed retry: only the once-rejected sound, as take 2
	if _, _, err := e.cli("-retry"); err != nil {
		t.Fatal(err)
	}
	if n := e.api.generations(); n != 2 {
		t.Fatalf("%d generations after -retry, want 2", n)
	}
	l := e.readLedger()
	if i := l.latest("animal.once.call", statePending); i < 0 || l.Takes[i].Take != 2 {
		t.Fatalf("retry take missing: %+v", l.Takes)
	}
	if _, _, err := e.cli("-retry"); err != nil || e.api.generations() != 2 {
		t.Fatal("a second retry was made")
	}
}

func TestRetriesOn429(t *testing.T) {
	t.Parallel()
	e := newEnv(t, row{"animal.cow.call", 1})
	e.api = newFakeAPI(t)
	e.api.script = []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusOK}
	if _, _, err := e.cli(); err != nil {
		t.Fatal(err)
	}
	if n := e.api.generations(); n != 3 {
		t.Fatalf("%d requests, want 3 (two failures then a take)", n)
	}
	if got := len(e.readLedger().Takes); got != 1 {
		t.Fatalf("failed attempts counted as takes: %d", got)
	}
	// still failing after three retries: give up, log nothing
	e2 := newEnv(t, row{"animal.cow.call", 1})
	e2.api = newFakeAPI(t)
	e2.api.script = []int{429, 429, 429, 429, 429}
	_, _, err := e2.cli()
	if err == nil || !strings.Contains(err.Error(), "gave up after 3 retries") {
		t.Fatalf("err = %v", err)
	}
	if n := e2.api.generations(); n != 4 {
		t.Fatalf("%d requests, want 1 + 3 retries", n)
	}
	if len(e2.readLedger().Takes) != 0 {
		t.Fatal("a failed call became a take")
	}
}

func TestFailsOn401And402(t *testing.T) {
	t.Parallel()
	for code, want := range map[int]string{401: "rejected the API key (HTTP 401)", 402: "(HTTP 402)", 403: "(HTTP 403)"} {
		e := newEnv(t, row{"animal.cow.call", 1}, row{"animal.cow.hurt", 1})
		e.api = newFakeAPI(t)
		e.api.script = []int{code}
		e.api.echoKey = true
		_, _, err := e.cli()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("HTTP %d: err = %v, want %q", code, err, want)
		}
		if strings.Contains(err.Error(), fakeKey) {
			t.Fatalf("HTTP %d: the error leaks the key: %v", code, err)
		}
		if n := e.api.generations(); n != 1 {
			t.Fatalf("HTTP %d: %d requests, a refusal must not retry or continue", code, n)
		}
		if len(e.readLedger().Takes) != 0 {
			t.Fatalf("HTTP %d: logged a take", code)
		}
	}
	var fe fatalError
	if !errors.As(fatalError{"x"}, &fe) {
		t.Fatal("fatalError")
	}
}

func TestMissingKeyRefused(t *testing.T) {
	t.Parallel()
	e := newEnv(t, row{"animal.cow.call", 1})
	e.api = newFakeAPI(t)
	e.key = ""
	if _, _, err := e.cli(); err == nil || !strings.Contains(err.Error(), keyEnv) {
		t.Fatalf("err = %v", err)
	}
	if e.api.generations() != 0 {
		t.Fatal("called the API without a key")
	}
}

func TestLedgerAppendsEveryTake(t *testing.T) {
	t.Parallel()
	e := newEnv(t, row{"animal.cow.call", 2}, row{"animal.cow.hurt", 0.6})
	e.api = newFakeAPI(t)
	old := Take{ID: "amb.day", Take: 1, Part: "1 of 4", File: "raw/old.mp3", Tool: "text_to_sound_effects", Seconds: 5, Loop: true, Credits: 50, Approved: "arxdsilva", Date: "2026-10-21"}
	e.writeLedger(old)
	if _, _, err := e.cli(); err != nil {
		t.Fatal(err)
	}
	l := e.readLedger()
	if len(l.Takes) != 3 || l.Takes[0] != old || l.Note != "test ledger" || !strings.Contains(string(l.Totals), "600") {
		t.Fatalf("the existing ledger was not preserved: %+v", l)
	}
	got := l.Takes[1]
	want := Take{ID: "animal.cow.call", Take: 1, File: "raw/animal.cow.call_t1.mp3", Tool: "text_to_sound_effects",
		Seconds: 2, Credits: 20, Prompt: "prompt for animal.cow.call, dry"}
	got.Date = ""
	if got != want {
		t.Fatalf("take = %+v, want %+v", got, want)
	}
	if l.Takes[2].Credits != 6 || l.Takes[2].Take != 1 {
		t.Fatalf("second take = %+v", l.Takes[2])
	}
	b, err := os.ReadFile(filepath.Join(e.raw, "animal.cow.call_t1.mp3"))
	if err != nil || !strings.HasPrefix(string(b), "FAKE-MP3:prompt for animal.cow.call, dry") {
		t.Fatalf("raw file = %q, %v", b, err)
	}
	// the request body: prompt + suffix, seconds, influence
	g := e.api.gens[0]
	if g["text"] != "prompt for animal.cow.call, dry" || g["duration_seconds"] != 2.0 || g["prompt_influence"] != 0.55 {
		t.Fatalf("request = %v", g)
	}
	// a re-run: the take is waiting for review, so nothing is generated again
	if _, _, err := e.cli(); err != nil || e.api.generations() != 2 {
		t.Fatalf("re-run spent credits: %v", err)
	}
}

func TestSecondsOverrideForTheSketch(t *testing.T) {
	t.Parallel()
	e := newEnv(t, row{"animal.cow.hurt", 0.6})
	e.api = newFakeAPI(t)
	if _, _, err := e.cli("-only", "animal.cow.hurt", "-seconds", "1", "-budget-from-api"); err != nil {
		t.Fatal(err)
	}
	if g := e.api.gens[0]; g["duration_seconds"] != 1.0 {
		t.Fatalf("request = %v", g)
	}
	if tk := e.readLedger().Takes[0]; tk.Seconds != 1 || tk.Credits != 20 {
		t.Fatalf("take = %+v (measured delta is 20)", tk)
	}
	if e.api.subs != 2 {
		t.Fatalf("%d subscription reads, want before and after", e.api.subs)
	}
}

// A lagging counter that jumps by several takes at once must not be
// charged again on top of the estimates already logged for them.
func TestLaggingCounterNotCountedTwice(t *testing.T) {
	t.Parallel()
	e := newEnv(t, threeSounds()...)
	e.api = newFakeAPI(t)
	e.api.lag = 3 // the counter shows nothing until the third take, then all 60
	if _, _, err := e.cli("-budget-from-api"); err != nil {
		t.Fatal(err)
	}
	l := e.readLedger()
	if len(l.Takes) != 3 || l.spent() != 60 {
		t.Fatalf("ledger logged %d credits over %d takes, want the 60 really spent", l.spent(), len(l.Takes))
	}
}

func TestNeverLogsKey(t *testing.T) {
	t.Parallel()
	e := newEnv(t, threeSounds()...)
	e.api = newFakeAPI(t)
	var all strings.Builder
	for _, args := range [][]string{{"-dry-run"}, {"-budget-from-api", "-max-credits", "30"}, {"-process"}, {"-review", "-o", filepath.Join(e.dir, "r.html")}} {
		out, errOut, err := e.cli(args...)
		all.WriteString(out + errOut)
		if err != nil {
			all.WriteString(err.Error())
		}
	}
	e.api.script = []int{401}
	e.api.echoKey = true
	_, errOut, err := e.cli("-only", "animal.cow.die")
	all.WriteString(errOut)
	if err != nil {
		all.WriteString(err.Error())
	}
	if strings.Contains(all.String(), fakeKey) {
		t.Fatalf("output leaks the key:\n%s", all.String())
	}
	filepath.Walk(e.dir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), fakeKey) {
				t.Errorf("%s contains the key", p)
			}
		}
		return nil
	})
	// the key did reach the API, as the header only
	e.api.mu.Lock()
	defer e.api.mu.Unlock()
	if len(e.api.keys) == 0 || e.api.keys[0] != fakeKey {
		t.Fatalf("keys sent = %v", e.api.keys)
	}
}

func TestRejectsUnknownSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"version":   `{"version":2,"defaults":{},"sounds":[]}`,
		"field":     `{"version":1,"surprise":1,"defaults":{},"sounds":[]}`,
		"id":        `{"version":1,"defaults":{},"sounds":[{"id":"../x","seconds":1,"prompt":"p"}]}`,
		"event":     `{"version":1,"defaults":{},"sounds":[{"id":"animal.cow.moo","seconds":1,"prompt":"p"}]}`,
		"seconds":   `{"version":1,"defaults":{},"sounds":[{"id":"animal.cow.call","seconds":99,"prompt":"p"}]}`,
		"duplicate": `{"version":1,"defaults":{},"sounds":[{"id":"animal.cow.call","seconds":1,"prompt":"p"},{"id":"animal.cow.call","seconds":1,"prompt":"p"}]}`,
		"empty":     `{"version":1,"defaults":{},"sounds":[{"id":"animal.cow.call","seconds":1,"prompt":" "}]}`,
		"influence": `{"version":1,"defaults":{"prompt_influence":3},"sounds":[]}`,
	} {
		f := filepath.Join(dir, name+".json")
		os.WriteFile(f, []byte(body), 0o644)
		if _, err := loadPrompts(f); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := loadPrompts(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file accepted")
	}
}

func TestSelectAndModes(t *testing.T) {
	t.Parallel()
	e := newEnv(t, threeSounds()...)
	if _, _, err := e.cli("-dry-run", "-only", "animal.cow.typo"); err == nil {
		t.Fatal("a pattern matching nothing was accepted")
	}
	if _, _, err := e.cli("-process", "-review"); err == nil {
		t.Fatal("two modes accepted")
	}
	if _, _, err := e.cli("-nope"); err == nil {
		t.Fatal("unknown flag accepted")
	}
	if _, _, err := e.cli("-h"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.cli("-approve", "animal.cow.call"); err == nil {
		t.Fatal("approving an id that is not in the ledger")
	}
	if _, _, err := e.cli("-reject", "animal.cow.call"); err == nil {
		t.Fatal("rejecting an id that is not in the ledger")
	}
}

func TestLedgerRoundTripKeepsRealFile(t *testing.T) {
	t.Parallel()
	const real = "../../assets/source/audio/elevenlabs/ledger.json"
	want, err := os.ReadFile(real)
	if err != nil {
		t.Skip(err)
	}
	l, err := loadLedger(real)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "ledger.json")
	if err := l.save(out); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != string(want) {
		t.Fatal("saving the real ledger would rewrite it")
	}
}
