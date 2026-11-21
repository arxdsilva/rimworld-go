// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/outbound/modloader"
	"rimworld-go/internal/adapters/outbound/wasmhost"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/modstate"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/domain/sim/clock"
)

// tallyContent is the run content of a plugin folder holding only tally
// (testdata/rust/tally), read by the real plugin loader.
func tallyContent(t *testing.T) runContent {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tally", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id": "tally", "name": "Tally", "version": "1.0.0", "game": ["0.1"]}`
	if err := os.WriteFile(filepath.Join(dir, "tally", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tally", "scripts", "tally.wasm"), tallyWasm(t), 0o644); err != nil {
		t.Fatal(err)
	}
	return buildContent(modloader.New(dir), []string{"tally"}, "0.1.35")
}

// realMux is scriptHost over c (the default engine), closed with the test.
func realMux(t *testing.T, c runContent) *scriptMux {
	t.Helper()
	m := scriptHost(c, false)
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// tallyMux is a mux over tally, then extra, on the interpreter.
func tallyMux(t *testing.T, extra ...modding.Plugin) *scriptMux {
	t.Helper()
	return testMux(t, append([]modding.Plugin{wasmPlugin("tally", "scripts/tally.wasm", tallyWasm(t))}, extra...)...)
}

// colony is a start-kit colony of seed, as runColonyHost plays it.
func colony(t *testing.T, seed uint64) *appgame.Game {
	t.Helper()
	g := appgame.New(appgame.Config{Seed: seed, Width: 32, Height: 32})
	if err := g.LoadStartKit(); err != nil {
		t.Fatal(err)
	}
	return g
}

func hours(g *appgame.Game, n int) { g.Step(n * clock.TicksPerHour) }

// started is g as the wizard's builder hands it over through withScripts(m).
func started(t *testing.T, m *scriptMux, g *appgame.Game) *appgame.Game {
	t.Helper()
	cfg := withScripts(uiadapter.AppConfig{NewSimBuilder: func(uiadapter.NewGame) (uiadapter.SimBuilder, error) {
		return stubBuilder{g}, nil
	}}, m)
	b, err := cfg.NewSimBuilder(uiadapter.NewGame{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Sim(); err != nil {
		t.Fatal(err)
	}
	return g
}

// loadHooks are withScripts(m)'s hooks around a load of data.
func loadHooks(m *scriptMux, data []byte) uiadapter.AppConfig {
	return withScripts(uiadapter.AppConfig{Saves: uiadapter.SaveHooks{Load: func(string) (uiadapter.Simulation, ports.SaveMeta, error) {
		g, _, err := appgame.LoadContent(bytes.NewReader(data), appgame.Content{})
		if err != nil {
			return nil, ports.SaveMeta{}, err
		}
		return g, ports.SaveMeta{}, nil
	}}}, m)
}

// loaded is data loaded through withScripts(m)'s load wrapper.
func loaded(t *testing.T, m *scriptMux, data []byte) *appgame.Game {
	t.Helper()
	sim, _, err := loadHooks(m, data).Saves.Load("slot")
	if err != nil {
		t.Fatal(err)
	}
	return sim.(*appgame.Game)
}

func saved(t *testing.T, g *appgame.Game) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "slot"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// blobSource is a fixed mod state for a save.
type blobSource []modstate.Blob

func (s blobSource) SaveBlobs() []modstate.Blob { return s }

// savedWith is a new colony's save holding blobs as its mod state.
func savedWith(t *testing.T, blobs ...modstate.Blob) []byte {
	t.Helper()
	g := colony(t, 3)
	appgame.UseModState(g, blobSource(blobs))
	return saved(t, g)
}

// takeState is the mod state data holds.
func takeState(t *testing.T, data []byte) []modstate.Blob {
	t.Helper()
	g, _, err := appgame.LoadContent(bytes.NewReader(data), appgame.Content{})
	if err != nil {
		t.Fatal(err)
	}
	bs, _ := appgame.TakeLoadedModState(g)
	return bs
}

const loadNotice = "Plugin mod state on load: "

// noticeCount is how many in-game messages start with head, and the last
// one's text.
func noticeCount(g *appgame.Game, head string) (int, string) {
	n, text := 0, ""
	for _, m := range g.World().Messages {
		if strings.HasPrefix(m.Text, head) {
			n, text = n+1, m.Text
		}
	}
	return n, text
}

// C4 (AC-2): playing 3 h, saving, loading and playing 3 h more equals 6 h
// straight, on the hash and on the mod state; without the restore it does
// not.
func TestTallyContinueEqualsContinuous(t *testing.T) {
	t.Parallel()
	c := tallyContent(t)
	m1 := realMux(t, c)
	g1 := started(t, m1, colony(t, 11))
	hours(g1, 6)
	m2 := realMux(t, c)
	g2 := started(t, m2, colony(t, 11))
	hours(g2, 3)
	data := saved(t, g2)
	g3 := loaded(t, m2, data)
	hours(g3, 3)
	if g1.StateHash() != g3.StateHash() {
		t.Fatalf("continued %x, continuous %x", g3.StateHash(), g1.StateHash())
	}
	if a, b := m1.SaveBlobs(), m2.SaveBlobs(); len(a) != 1 || !reflect.DeepEqual(a, b) {
		t.Fatalf("blobs: continuous %v, continued %v", a, b)
	}
	m3 := realMux(t, c) // control: the same load with no restore
	g4, _, err := appgame.LoadContent(bytes.NewReader(data), appgame.Content{})
	if err != nil {
		t.Fatal(err)
	}
	m3.start(g4)
	hours(g4, 3)
	if g4.StateHash() == g1.StateHash() {
		t.Fatal("a load without the restore gave the same colony: the test cannot see the restore")
	}
}

// C5 (AC-2): hashing and autosaving every hour change nothing.
func TestTallyHashAndAutosaveChangeNothing(t *testing.T) {
	t.Parallel()
	run := func(probe bool) (uint64, any, []modstate.Blob) {
		m := tallyMux(t)
		g := started(t, m, colony(t, 11))
		for range 6 {
			hours(g, 1)
			if probe {
				_ = g.StateHash()
				if err := appgame.EncodeSave(g, io.Discard, "autosave"); err != nil {
					t.Fatal(err)
				}
			}
		}
		return g.StateHash(), m.Stats(), m.SaveBlobs()
	}
	h1, s1, b1 := run(false)
	h2, s2, b2 := run(true)
	if h1 != h2 || !reflect.DeepEqual(s1, s2) || !reflect.DeepEqual(b1, b2) {
		t.Fatalf("probes changed the run: hash %x/%x, stats %+v/%+v, blobs %v/%v", h1, h2, s1, s2, b1, b2)
	}
}

// C6 (AC-2): a module whose save traps leaves the game save whole, without
// its state, with one Late problem, and the run unchanged.
func TestTrappingSaveKeepsTheGameSave(t *testing.T) {
	t.Parallel()
	m := tallyMux(t, wasmPlugin("trap", "scripts/trap.wasm", guest{hour: "t", state: true, trapSave: true}.bytes()))
	g := started(t, m, colony(t, 11))
	hours(g, 1)
	before := m.Stats()
	var data []byte
	for range 3 {
		data = saved(t, g)
	}
	if after := m.Stats(); !reflect.DeepEqual(before, after) || statOf(after, "trap").Off {
		t.Fatalf("saving changed the stats: %+v, then %+v", before, after)
	}
	late := m.Late()
	if len(late) != 1 || late[0].Plugin != "trap" || !strings.HasPrefix(late[0].Msg, "mod state not saved: ") {
		t.Fatalf("Late = %+v", late)
	}
	bs := takeState(t, data)
	if len(bs) != 1 || bs[0].Plugin != "tally" {
		t.Fatalf("the save's mod state: %+v", bs)
	}
}

// Ruling "off not saved": a file with state that a trap turned off is left
// out of the save without a save problem.
func TestTrappingSaveSkipsAnOffFile(t *testing.T) {
	t.Parallel()
	m := tallyMux(t, wasmPlugin("off", "scripts/off.wasm", guest{trapHour: true, state: true}.bytes()))
	hours(started(t, m, colony(t, 11)), 1)
	if !statOf(m.Stats(), "off").Off {
		t.Fatalf("the trapping file is on: %+v", m.Stats())
	}
	if bs := m.SaveBlobs(); len(bs) != 1 || bs[0].Plugin != "tally" {
		t.Fatalf("blobs: %+v", bs)
	}
	for _, p := range m.Late() {
		if strings.HasPrefix(p.Msg, "mod state not saved: ") {
			t.Fatalf("an off file is a save problem: %+v", p)
		}
	}
}

// stateBlob is a tally blob with the given state version and counter.
func stateBlob(version, counter uint32) []byte {
	le := binary.LittleEndian
	b := le.AppendUint32(le.AppendUint32(le.AppendUint32([]byte("RWMS"), wasmhost.ABIVersion), version), 4)
	return le.AppendUint32(b, counter)
}

// C7 (AC-2): a blob for a file that is not loaded, and one of another state
// version, are dropped with a Warn note and one in-game message; the file
// runs with fresh state; an older save loads with no note.
func TestModStateLoadNotices(t *testing.T) {
	t.Parallel()
	gone := savedWith(t, modstate.Blob{Plugin: "gone", File: "scripts/gone.wasm", Data: stateBlob(1, 7)})
	m := tallyMux(t)
	g := loaded(t, m, gone)
	late := m.Late()
	if len(late) != 1 || !late[0].Warn || late[0].Plugin != "gone" || late[0].Msg != "mod state dropped: file no longer loaded" {
		t.Fatalf("Late = %+v", late)
	}
	g.Step(2)
	if n, text := noticeCount(g, loadNotice); n != 1 || !strings.Contains(text, "gone/scripts/gone.wasm") {
		t.Fatalf("notices: %d %q", n, text)
	}

	m2 := tallyMux(t) // wrong state_version: rejected, the file runs fresh
	g2 := loaded(t, m2, savedWith(t, modstate.Blob{Plugin: "tally", File: "scripts/tally.wasm", Data: stateBlob(2, 40)}))
	late = m2.Late()
	if len(late) != 1 || !late[0].Warn || late[0].Plugin != "tally" || !strings.HasPrefix(late[0].Msg, "mod state not restored: ") {
		t.Fatalf("Late = %+v", late)
	}
	hours(g2, 2)
	m3 := tallyMux(t)
	g3 := loaded(t, m3, savedWith(t))
	hours(g3, 2)
	if a, b := m2.SaveBlobs(), m3.SaveBlobs(); statOf(m2.Stats(), "tally").Off || !reflect.DeepEqual(a, b) {
		t.Fatalf("rejected blob: %v, fresh %v, stats %+v", a, b, m2.Stats())
	}
	if n, _ := noticeCount(g2, loadNotice); n != 1 {
		t.Fatalf("notices: %d", n)
	}

	old, err := os.ReadFile(filepath.Join("..", "..", "application", "game", "testdata", "v12.sav")) // a pre-v18 save
	if err != nil {
		t.Fatal(err)
	}
	m4 := tallyMux(t)
	g4 := loaded(t, m4, old)
	g4.Step(2)
	if n, _ := noticeCount(g4, loadNotice); n != 0 || len(m4.Late()) != 0 {
		t.Fatalf("an old save: %d notices, Late %+v", n, m4.Late())
	}
}

// C8 (AC-5): a new game after a game on the same mux equals it on a fresh
// mux.
func TestNewGameAfterGame(t *testing.T) {
	t.Parallel()
	m := tallyMux(t)
	hours(started(t, m, colony(t, 11)), 4)
	b := started(t, m, colony(t, 12))
	hours(b, 3)
	f := tallyMux(t)
	fb := started(t, f, colony(t, 12))
	hours(fb, 3)
	if b.StateHash() != fb.StateHash() || !reflect.DeepEqual(m.SaveBlobs(), f.SaveBlobs()) || !reflect.DeepEqual(m.Stats(), f.Stats()) {
		t.Fatalf("after a game: hash %x, blobs %v; fresh: %x, %v", b.StateHash(), m.SaveBlobs(), fb.StateHash(), f.SaveBlobs())
	}
}

// C8 (AC-5): a load after a game equals the same load on a fresh mux.
func TestLoadAfterGame(t *testing.T) {
	t.Parallel()
	s := tallyMux(t)
	src := started(t, s, colony(t, 11))
	hours(src, 2)
	data := saved(t, src)
	m := tallyMux(t)
	hours(started(t, m, colony(t, 12)), 4)
	a := loaded(t, m, data)
	hours(a, 2)
	f := tallyMux(t)
	fa := loaded(t, f, data)
	hours(fa, 2)
	if a.StateHash() != fa.StateHash() || !reflect.DeepEqual(m.SaveBlobs(), f.SaveBlobs()) || !reflect.DeepEqual(m.Stats(), f.Stats()) {
		t.Fatalf("after a game: hash %x, blobs %v, stats %+v; fresh: %x, %v, %+v", a.StateHash(), m.SaveBlobs(), m.Stats(),
			fa.StateHash(), f.SaveBlobs(), f.Stats())
	}
}

// C10 (AC-4): tally installed in a plugin folder runs in a colony through
// the real loader, content and script host.
func TestTallyFromPluginDir(t *testing.T) {
	t.Parallel()
	m := realMux(t, tallyContent(t))
	g := started(t, m, colony(t, 11))
	hours(g, 4)
	_, plain := runColony(t, 11, 4)
	if wood := g.StockOf("wood"); wood <= plain {
		t.Fatalf("tally did not run: wood %d, plain %d", wood, plain)
	}
	if late := m.Late(); len(late) != 0 {
		t.Fatalf("problems: %+v", late)
	}
	if st := statOf(m.Stats(), "tally"); st.Calls == 0 || st.Fuel == 0 || st.Off {
		t.Fatalf("stats: %+v", st)
	}
}

// failingBuilder builds a game and still reports an error.
type failingBuilder struct{ g *appgame.Game }

func (b failingBuilder) Step(int) (string, float32, bool)   { return "", 1, true }
func (b failingBuilder) Sim() (uiadapter.Simulation, error) { return b.g, errors.New("no map") }

// C12 (AC-5): a failed load and a failed build leave the running game's
// mod state and counters as they were.
func TestFailedLoadKeepsRunningGame(t *testing.T) {
	t.Parallel()
	m := tallyMux(t)
	g := started(t, m, colony(t, 11))
	hours(g, 4)
	blobs, stats := m.SaveBlobs(), m.Stats()
	cfg := withScripts(uiadapter.AppConfig{
		Saves: uiadapter.SaveHooks{Load: func(path string) (uiadapter.Simulation, ports.SaveMeta, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, ports.SaveMeta{}, err
			}
			defer f.Close()
			lg, _, err := appgame.LoadContent(f, appgame.Content{})
			return lg, ports.SaveMeta{}, err
		}},
		NewSimBuilder: func(uiadapter.NewGame) (uiadapter.SimBuilder, error) { return failingBuilder{colony(t, 12)}, nil },
	}, m)
	if _, _, err := cfg.Saves.Load(filepath.Join(t.TempDir(), "missing.sav")); err == nil {
		t.Fatal("a missing file loaded")
	}
	b, err := cfg.NewSimBuilder(uiadapter.NewGame{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Sim(); err == nil {
		t.Fatal("the failing builder built")
	}
	if !reflect.DeepEqual(blobs, m.SaveBlobs()) || !reflect.DeepEqual(stats, m.Stats()) {
		t.Fatalf("failures reset the mux: blobs %v then %v, stats %+v then %+v", blobs, m.SaveBlobs(), stats, m.Stats())
	}
}

// C13 (AC-2): with no scripted plugin, a save's mod state is dropped with
// notes and one message, the tick path is today's, and the next save has
// no mod state.
func TestModStateDroppedWithNoScriptPlugins(t *testing.T) {
	t.Parallel()
	m := realMux(t, buildContent(modloader.New(t.TempDir()), nil, "0.1.35"))
	if m.host() != nil || m.source() != nil {
		t.Fatal("no scripted plugin: no host and no mod state source go to the game")
	}
	plain, _ := runColony(t, 11, 2)
	g := started(t, m, colony(t, 11))
	hours(g, 2)
	if g.StateHash() != plain {
		t.Fatal("an empty mux changed the colony")
	}
	data := savedWith(t, modstate.Blob{Plugin: "a", File: "scripts/a.wasm", Data: []byte{1}},
		modstate.Blob{Plugin: "b", File: "scripts/b.wasm", Data: []byte{2}})
	cfg := loadHooks(m, data)
	if cfg.Plugins.Late == nil || cfg.Plugins.Stats != nil {
		t.Fatal("Late is wired for every run; Stats only with a Wasm host")
	}
	sim, _, err := cfg.Saves.Load("slot")
	if err != nil {
		t.Fatal(err)
	}
	lg := sim.(*appgame.Game)
	late := cfg.Plugins.Late()
	if len(late) != 2 || !late[0].Warn || !late[1].Warn || late[0].Plugin != "a" || late[1].Plugin != "b" ||
		late[0].Msg != "mod state dropped: plugin or file not loaded" {
		t.Fatalf("Late = %+v", late)
	}
	lg.Step(2)
	if n, text := noticeCount(lg, loadNotice); n != 1 || !strings.Contains(text, "a/scripts/a.wasm") || !strings.Contains(text, "b/scripts/b.wasm") {
		t.Fatalf("notices: %d %q", n, text)
	}
	if bs := takeState(t, saved(t, lg)); len(bs) != 0 {
		t.Fatalf("the next save kept mod state: %+v", bs)
	}
}

// C14 (N7): Stats show zeros right after Reset and the restore's load call
// right after a load, with no Fire.
func TestMuxStatsAfterResetAndRestore(t *testing.T) {
	t.Parallel()
	m := tallyMux(t)
	g := started(t, m, colony(t, 11))
	hours(g, 3)
	if st := statOf(m.Stats(), "tally"); st.Calls == 0 {
		t.Fatalf("no calls after 3 h: %+v", st)
	}
	data := saved(t, g)
	m.Reset()
	if st := statOf(m.Stats(), "tally"); st.Calls != 0 || st.Fuel != 0 || st.Off || st.Plugin != "tally" {
		t.Fatalf("after Reset: %+v", st)
	}
	loaded(t, m, data)
	if st := statOf(m.Stats(), "tally"); st.Calls != 1 || st.Fuel == 0 {
		t.Fatalf("after the restore: %+v", st)
	}
}

// W1-game C9 relay, lead ruling save-notes: a blob the game's save refuses
// (a file name over modstate.MaxName) is a Warn note on Late, not repeated
// by later saves, and no in-game message: the run (state hash and message
// log) equals one that never saved. The save keeps the other files' state.
func TestModStateSaveNotices(t *testing.T) {
	t.Parallel()
	long := "scripts/" + strings.Repeat("x", modstate.MaxName) + ".wasm"
	run := func(saves int) (*appgame.Game, *scriptMux, []byte) {
		m := tallyMux(t, wasmPlugin("long", long, guest{hour: "l", state: true}.bytes()))
		g := colony(t, 11)
		var buf bytes.Buffer
		cfg := withScripts(uiadapter.AppConfig{
			NewSimBuilder: func(uiadapter.NewGame) (uiadapter.SimBuilder, error) { return stubBuilder{g}, nil },
			Saves: uiadapter.SaveHooks{Save: func(sim uiadapter.Simulation, _ ports.SaveMeta) error {
				buf.Reset()
				return sim.(*appgame.Game).Save(&buf, "slot")
			}},
		}, m)
		b, err := cfg.NewSimBuilder(uiadapter.NewGame{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.Sim(); err != nil {
			t.Fatal(err)
		}
		hours(g, 1)
		for range saves {
			if err := cfg.Saves.Save(g, ports.SaveMeta{}); err != nil {
				t.Fatal(err)
			}
		}
		hours(g, 2)
		return g, m, buf.Bytes()
	}
	g, m, data := run(3)
	late := m.Late()
	if len(late) != 1 || !late[0].Warn || late[0].Plugin != "long" || !strings.Contains(late[0].Msg, "too long") {
		t.Fatalf("Late = %+v", late)
	}
	plain, pm, _ := run(0)
	if len(pm.Late()) != 0 {
		t.Fatalf("a run that never saved has notes: %+v", pm.Late())
	}
	if g.StateHash() != plain.StateHash() || !reflect.DeepEqual(g.World().Messages, plain.World().Messages) {
		t.Fatalf("saves changed the run: hash %x/%x, messages %d/%d", g.StateHash(), plain.StateHash(),
			len(g.World().Messages), len(plain.World().Messages))
	}
	if bs := takeState(t, data); len(bs) != 1 || bs[0].Plugin != "tally" {
		t.Fatalf("the save's mod state: %+v", bs)
	}
}
