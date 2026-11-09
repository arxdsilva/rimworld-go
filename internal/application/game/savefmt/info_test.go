package savefmt

import (
	"bytes"
	"slices"
	"errors"
	"io"
	"testing"
)

// An older header gets its colony from its name: an autosave belongs to
// the slot it is named after.
func TestUpgradeNamesTheColonyOfAnOldHeader(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, base string
		auto       bool
	}{
		{"colony-2026", "colony-2026", false},
		{"colony-2026-autosave", "colony-2026", true},
		{"colony-2026-autosave-3", "colony-2026", true},
	} {
		i := Info{Version: 15, Name: c.name}
		i.Upgrade()
		want := Info{Version: 15, Name: c.name, Base: c.base, Autosave: c.auto, Colony: "slot:" + c.base, ColonyName: c.base}
		if i.Name != want.Name || i.Base != want.Base || i.Autosave != want.Autosave || i.Colony != want.Colony || i.ColonyName != want.ColonyName || i.Version != 15 {
			t.Errorf("%q upgraded to %+v, want %+v", c.name, i, want)
		}
	}
	// A v16 header written without a Stamp names no colony: it gets one the
	// same way.
	i := Info{Version: InfoVersion, Name: "a-autosave"}
	if i.Upgrade(); i.Colony != "slot:a" || i.Base != "a" || !i.Autosave || i.ColonyName != "a" {
		t.Errorf("an unstamped v16 header upgraded to %+v", i)
	}
	// A header that names its colony, old or new, is kept.
	for _, i := range []Info{
		{Version: InfoVersion, Name: "a-autosave", Colony: "c2", Base: "x"},
		{Version: 3, Name: "a-autosave", Colony: "c1", Base: "a", ColonyName: "Ada"},
	} {
		before := i
		i.Upgrade()
		if i.Colony != before.Colony || i.Base != before.Base || i.ColonyName != before.ColonyName || i.Autosave != before.Autosave {
			t.Errorf("%+v changed to %+v", before, i)
		}
	}
}

func TestBaseSlotDropsTheAutosaveSuffix(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"colony-autosave-3": "colony", "colony-autosave": "colony", "colony": "colony", "a-autosave-b": "a-autosave-b",
		"x-autosave-12": "x", "": "",
	} {
		if got := BaseSlot(in); got != want {
			t.Errorf("BaseSlot(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlotKeyKeepsFileSafeRunes(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"  My Colony 2 ": "My-Colony-2", "a/b\\c:d": "abcd", "": "colony", "Ünïcode": "ncode", "x_y-z": "x_y-z", "***": "colony",
	} {
		if got := SlotKey(in); got != want {
			t.Errorf("SlotKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStampFillsTheClientFields(t *testing.T) {
	t.Parallel()
	i := Info{Version: InfoVersion, Name: "old", Colonists: 3}
	Stamp{Name: "n", Colony: "c", ColonyName: "Ada", Base: "b", Autosave: true, Season: "Spring", Build: "v1", Content: Content{Hash: 7, Plugins: []string{"p"}}}.Apply(&i)
	if i.Name != "n" || i.Colony != "c" || i.ColonyName != "Ada" || i.Base != "b" || !i.Autosave || i.Season != "Spring" || i.Build != "v1" ||
		i.Content.Hash != 7 || len(i.Content.Plugins) != 1 || i.Colonists != 3 {
		t.Fatalf("stamped %+v", i)
	}
}

func TestCheckNameAndCompat(t *testing.T) {
	t.Parallel()
	if CheckName("  ") == "" || CheckName("Base camp") != "" {
		t.Fatal("a blank name is refused, a plain one taken")
	}
	if CompatOK != 0 {
		t.Fatal("CompatOK is the zero value (ports.SaveSlot.Compat: 0 fine)")
	}
}

// Drain checks the checksum at the stream's end: a file cut short is
// ErrCorrupt.
func TestDrainFindsACutFile(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Compress(&buf, bytes.Repeat([]byte("colony "), 4000)); err != nil {
		t.Fatal(err)
	}
	zr, err := NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil || Drain(zr) != nil {
		t.Fatalf("a whole file: %v", err)
	}
	zr, err = NewReader(bytes.NewReader(buf.Bytes()[:buf.Len()-6]))
	if err != nil {
		t.Fatal(err)
	}
	if err := Drain(zr); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("a cut file drained with %v, want ErrCorrupt", err)
	}
	if err := Drain(io.LimitReader(bytes.NewReader(nil), 0)); err != nil {
		t.Fatalf("an empty reader: %v", err)
	}
}

func TestClampCutsRunes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{{"colony", 3, "col"}, {"colony", 10, "colony"}, {"äöü", 2, "äö"}, {"x", 0, ""}, {"", 4, ""}} {
		if got := Clamp(c.in, c.n); got != c.want {
			t.Errorf("Clamp(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestCompatOf(t *testing.T) {
	t.Parallel()
	rt := Runtime{Version: 16, Min: 2, Build: "v1.2", Content: Content{Hash: 7, Plugins: []string{"core"}}}
	same := Content{Hash: 7, Plugins: []string{"core"}}
	for _, c := range []struct {
		name string
		info Info
		want Compat
	}{
		{"this build", Info{Version: 16, Build: "v1.2", Content: same}, CompatOK},
		{"another build's content", Info{Version: 16, Build: "v1.1", Content: Content{Hash: 9}}, CompatOK},
		{"this build, other content", Info{Version: 16, Build: "v1.2", Content: Content{Hash: 9}}, CompatContent},
		{"older format", Info{Version: 15, Build: "v1.2", Content: Content{Hash: 9}}, CompatOlder},
		{"oldest read", Info{Version: 2}, CompatOlder},
		{"too old", Info{Version: 1}, CompatTooOld},
		{"no version", Info{}, CompatTooOld},
		{"newer format", Info{Version: 17, Build: "v1.2", Content: same}, CompatNewer},
	} {
		got := CompatOf(c.info, rt)
		if got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
		if got.Loads() != (c.want != CompatNewer && c.want != CompatTooOld) {
			t.Errorf("%s: Loads %v", c.name, got.Loads())
		}
	}
}

func TestMissingPlugins(t *testing.T) {
	t.Parallel()
	rt := Runtime{Content: Content{Plugins: []string{"core"}}}
	if got := MissingPlugins(Info{Content: Content{Plugins: []string{"core", "moreFauna", "x"}}}, rt); !slices.Equal(got, []string{"moreFauna", "x"}) {
		t.Fatalf("missing %v", got)
	}
	if got := MissingPlugins(Info{}, rt); got != nil {
		t.Fatalf("a header with no plugins misses %v", got)
	}
}

// A byte flipped in the middle of the compressed stream is caught by the
// time the stream ends: as bad deflate data or by the CRC.
func TestDrainDetectsFlippedByte(t *testing.T) {
	t.Parallel()
	raw := make([]byte, 1<<16)
	x := uint32(1)
	for i := range raw {
		x = x*1664525 + 1013904223
		raw[i] = "colony pawns build walls "[x>>27%25]
	}
	var buf bytes.Buffer
	if err := Compress(&buf, raw); err != nil {
		t.Fatal(err)
	}
	for _, at := range []int{buf.Len() / 3, buf.Len() / 2, 2 * buf.Len() / 3} {
		bad := bytes.Clone(buf.Bytes())
		bad[at] ^= 0x10
		zr, err := NewReader(bytes.NewReader(bad))
		if err != nil {
			t.Fatal(err)
		}
		if err := Drain(zr); !errors.Is(err, ErrCorrupt) {
			t.Errorf("byte %d of %d flipped: drained with %v, want ErrCorrupt", at, buf.Len(), err)
		}
	}
}

func TestFinishWrapsDecodeErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("gob: bad data")
	if err := Finish(boom, bytes.NewReader(nil)); !errors.Is(err, ErrCorrupt) || !errors.Is(err, boom) {
		t.Fatalf("decode error became %v", err)
	}
	if err := Finish(nil, bytes.NewReader([]byte("rest"))); err != nil {
		t.Fatalf("a clean body: %v", err)
	}
}
