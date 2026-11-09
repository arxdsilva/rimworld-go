package filesave

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
)

// writeDamaged copies src to name in dir with damage applied.
func writeDamaged(t *testing.T, dir, name string, src []byte, damage func([]byte) []byte) string {
	t.Helper()
	path := filepath.Join(dir, name+Ext)
	if err := os.WriteFile(path, damage(bytes.Clone(src)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func slotsByName(t *testing.T, s Store, v *Verifier) map[string]Slot {
	t.Helper()
	slots, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	v.Verify(slots)
	out := map[string]Slot{}
	for _, sl := range slots {
		out[sl.Name] = sl
	}
	return out
}

func TestListMarksCorruptBody(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	good, err := s.Save(game.NewColony(game.Config{Seed: 6, Width: 60, Height: 60}), "good")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	flipped := writeDamaged(t, s.Dir, "flipped", data, func(b []byte) []byte { b[len(b)/2] ^= 0x10; return b })
	writeDamaged(t, s.Dir, "cut", data, func(b []byte) []byte { return b[:len(b)-len(b)/4] })
	writeDamaged(t, s.Dir, "crc", data, func(b []byte) []byte { b[len(b)-6] ^= 0xff; return b })

	v := NewVerifier()
	got := slotsByName(t, s, v)
	if len(got) != 4 || got["good"].Err != nil {
		t.Fatalf("slots %+v", got)
	}
	for _, name := range []string{"flipped", "cut", "crc"} {
		if !errors.Is(got[name].Err, savefmt.ErrCorrupt) {
			t.Errorf("%s: Err %v, want ErrCorrupt", name, got[name].Err)
		}
	}
	if _, _, err := s.Load(flipped); !errors.Is(err, savefmt.ErrCorrupt) {
		t.Errorf("Load of a flipped body: %v, want ErrCorrupt", err)
	}

	// The memo: a file of the same size and time is not read again; a new
	// time is.
	fi, err := os.Stat(good)
	if err != nil {
		t.Fatal(err)
	}
	damaged := bytes.Clone(data)
	damaged[len(damaged)/2] ^= 0x10
	if err := os.WriteFile(good, damaged, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(good, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := slotsByName(t, s, v); got["good"].Err != nil {
		t.Fatalf("an unchanged size and time was read again: %v", got["good"].Err)
	}
	later := fi.ModTime().Add(time.Minute)
	if err := os.Chtimes(good, later, later); err != nil {
		t.Fatal(err)
	}
	if got := slotsByName(t, s, v); !errors.Is(got["good"].Err, savefmt.ErrCorrupt) {
		t.Fatalf("a changed file was not read again: %v", got["good"].Err)
	}
}

// A save from a newer build keeps its header in the list (the list says
// "newer", not "unreadable"), and Verify leaves it alone.
func TestListKeepsNewerHeaderVersion(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	var raw bytes.Buffer
	head := savefmt.Info{Version: game.SaveVersion + 1, Name: "From the future", Colony: "c9", ColonyName: "Later", Base: "From the future", Build: "v9"}
	if err := gob.NewEncoder(&raw).Encode(head); err != nil {
		t.Fatal(err)
	}
	raw.WriteString("a body this build cannot read")
	var file bytes.Buffer
	if err := savefmt.Compress(&file, raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "future"+Ext), file.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	sl := slotsByName(t, s, NewVerifier())["future"]
	if !errors.Is(sl.Err, game.ErrSaveVersion) || errors.Is(sl.Err, savefmt.ErrCorrupt) {
		t.Fatalf("Err %v, want ErrSaveVersion", sl.Err)
	}
	if sl.Info.Version != head.Version || sl.Info.Name != head.Name || sl.Info.Colony != "c9" || sl.Info.Build != "v9" {
		t.Fatalf("header %+v, want %+v", sl.Info, head)
	}
	rt := savefmt.Runtime{Version: game.SaveVersion, Min: 1}
	if c := savefmt.CompatOf(sl.Info, rt); c != savefmt.CompatNewer {
		t.Fatalf("compat %d, want newer", c)
	}
}

// BenchmarkListVerify10Large is the load list opening on ten saves of a
// 500x500 colony nobody has verified yet (SAV-022 gate: 300 ms).
func BenchmarkListVerify10Large(b *testing.B) {
	s := Store{Dir: b.TempDir()}
	first, err := s.Save(game.NewColony(game.Config{Seed: 9, Width: 500, Height: 500}), "big0")
	if err != nil {
		b.Fatal(err)
	}
	data, err := os.ReadFile(first)
	if err != nil {
		b.Fatal(err)
	}
	for i := 1; i < 10; i++ {
		if err := os.WriteFile(filepath.Join(s.Dir, "big"+string(rune('0'+i))+Ext), data, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for b.Loop() {
		slots, err := s.List()
		if err != nil || len(slots) != 10 {
			b.Fatalf("slots %d, %v", len(slots), err)
		}
		NewVerifier().Verify(slots)
		for _, sl := range slots {
			if sl.Err != nil {
				b.Fatal(sl.Err)
			}
		}
	}
	b.ReportMetric(float64(len(data))/1e6, "MB/save")
}
