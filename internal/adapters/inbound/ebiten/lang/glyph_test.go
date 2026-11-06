package lang

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// shippedCatalogs decodes every assets/lang/*.json as source→translation
// (the _meta entry left out); a malformed file fails the test.
func shippedCatalogs(t *testing.T, dir string) map[string]map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatalf("%s: %v", filepath.Base(p), err)
		}
		m := map[string]string{}
		for k, v := range raw {
			if k == metaKey {
				continue
			}
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				t.Fatalf("%s %q: %v", filepath.Base(p), k, err)
			}
			m[k] = s
		}
		out[strings.TrimSuffix(filepath.Base(p), ".json")] = m
	}
	return out
}

// TestCatalogGlyphs reports the runes a catalog uses that the bundled
// Noto Sans faces cannot draw (a CJK language needs its own font,
// OPTIONS_GAP_ANALYSIS.md "Fonts"). English is the source text, so a
// missing rune there fails; other catalogs are reported.
func TestCatalogGlyphs(t *testing.T) {
	t.Parallel()
	fonts := map[string]cmap{}
	for _, name := range []string{"NotoSans-Regular.ttf", "NotoSans-SemiBold.ttf"} {
		b, err := os.ReadFile(filepath.Join("..", "gfx", "fontdata", name))
		if err != nil {
			t.Fatal(err)
		}
		cm, err := parseCmap(b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fonts[name] = cm
	}
	dir := filepath.Join(repoRoot(t), "assets", "lang")
	for code, m := range shippedCatalogs(t, dir) {
		missing := map[rune]bool{}
		for k, v := range m {
			for _, r := range k + v {
				if unicode.IsControl(r) {
					continue
				}
				for _, cm := range fonts {
					if !cm.has(r) {
						missing[r] = true
					}
				}
			}
		}
		if len(missing) == 0 {
			continue
		}
		rs := make([]rune, 0, len(missing))
		for r := range missing {
			rs = append(rs, r)
		}
		slices.Sort(rs)
		var b strings.Builder
		for _, r := range rs {
			fmt.Fprintf(&b, " %c(U+%04X)", r, r)
		}
		if code == EnglishCode {
			t.Errorf("en.json uses runes the UI font lacks:%s", b.String())
		} else {
			t.Logf("%s.json uses %d runes the UI font lacks:%s", code, len(rs), b.String())
		}
	}
}

func TestFontCoverage(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "gfx", "fontdata", "NotoSans-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	cm, err := parseCmap(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range "Aaéßçœ€—Ωж°" {
		if !cm.has(r) {
			t.Errorf("Noto Sans lacks %c (Latin, Greek and Cyrillic expected)", r)
		}
	}
	for _, r := range "日本語한" {
		if cm.has(r) {
			t.Errorf("Noto Sans claims %c; the CJK note in OPTIONS_GAP_ANALYSIS is stale", r)
		}
	}
}

// cmap is a font's character map: format 12 groups or format 4 segments.
type cmap struct {
	groups [][3]uint32 // format 12: start, end, start glyph
	f4     []byte      // format 4 subtable
}

// parseCmap finds the best Unicode cmap subtable of an sfnt font.
func parseCmap(b []byte) (cmap, error) {
	be := binary.BigEndian
	if len(b) < 12 {
		return cmap{}, errors.New("short font")
	}
	var tbl []byte
	for i, n := 0, int(be.Uint16(b[4:])); i < n; i++ {
		rec := b[12+16*i:]
		if string(rec[:4]) == "cmap" {
			off, ln := be.Uint32(rec[8:]), be.Uint32(rec[12:])
			tbl = b[off : off+ln]
		}
	}
	if tbl == nil {
		return cmap{}, errors.New("no cmap table")
	}
	var f4 []byte
	for i, n := 0, int(be.Uint16(tbl[2:])); i < n; i++ {
		rec := tbl[4+8*i:]
		plat, enc, sub := be.Uint16(rec), be.Uint16(rec[2:]), tbl[be.Uint32(rec[4:]):]
		unicodeMap := plat == 0 || (plat == 3 && (enc == 1 || enc == 10))
		switch format := be.Uint16(sub); {
		case unicodeMap && format == 12:
			var cm cmap
			for g, ng := 0, int(be.Uint32(sub[12:])); g < ng; g++ {
				e := sub[16+12*g:]
				cm.groups = append(cm.groups, [3]uint32{be.Uint32(e), be.Uint32(e[4:]), be.Uint32(e[8:])})
			}
			return cm, nil
		case unicodeMap && format == 4:
			f4 = sub[:be.Uint16(sub[2:])]
		}
	}
	if f4 == nil {
		return cmap{}, errors.New("no Unicode cmap subtable")
	}
	return cmap{f4: f4}, nil
}

// has reports whether the font maps r to a real glyph.
func (c cmap) has(r rune) bool {
	if c.groups != nil {
		for _, g := range c.groups {
			if uint32(r) >= g[0] && uint32(r) <= g[1] {
				return g[2]+uint32(r)-g[0] != 0
			}
		}
		return false
	}
	if r > 0xFFFF {
		return false
	}
	be, s, ch := binary.BigEndian, c.f4, uint16(r)
	segs := int(be.Uint16(s[6:])) / 2
	ends, starts := 14, 16+2*segs
	deltas, ranges := starts+2*segs, starts+4*segs
	for i := 0; i < segs; i++ {
		if be.Uint16(s[ends+2*i:]) < ch {
			continue
		}
		start := be.Uint16(s[starts+2*i:])
		if start > ch {
			return false
		}
		delta, ro := be.Uint16(s[deltas+2*i:]), int(be.Uint16(s[ranges+2*i:]))
		if ro == 0 {
			return ch+delta != 0
		}
		at := ranges + 2*i + ro + 2*int(ch-start)
		if at+2 > len(s) {
			return false
		}
		g := be.Uint16(s[at:])
		return g != 0 && g+delta != 0
	}
	return false
}
