package ebiten

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUITokensMatchPaletteYAML(t *testing.T) {
	t.Parallel()
	path, err := locateUIPaletteYAML()
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseUIPaletteYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(uiTokens) {
		t.Fatalf("yaml has %d tokens, go has %d", len(got), len(uiTokens))
	}
	for name, want := range uiTokens {
		c, ok := got[name]
		if !ok {
			t.Fatalf("yaml missing %s", name)
		}
		if c != want {
			t.Fatalf("%s yaml=%v go=%v", name, c, want)
		}
	}
}

func TestLerpRGBAClamps(t *testing.T) {
	t.Parallel()
	a := color.RGBA{R: 10, G: 20, B: 30, A: 40}
	b := color.RGBA{R: 110, G: 120, B: 130, A: 140}
	if got := lerpRGBA(a, b, 0); got != a {
		t.Fatalf("t=0: %v", got)
	}
	if got := lerpRGBA(a, b, 1); got != b {
		t.Fatalf("t=1: %v", got)
	}
	mid := lerpRGBA(a, b, 0.5)
	if mid.R != 60 || mid.G != 70 || mid.B != 80 || mid.A != 90 {
		t.Fatalf("t=0.5: %v", mid)
	}
}

func locateUIPaletteYAML() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	const rel = "assets/palettes/ui.yaml"
	for i := 0; i < 12; i++ {
		cand := filepath.Join(dir, rel)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("palette %q not found from cwd", rel)
}

func parseUIPaletteYAML(path string) (map[string]color.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]color.RGBA{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, hexstr, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("bad palette line %q", line)
		}
		key = strings.TrimSpace(key)
		hexstr = strings.Trim(strings.TrimSpace(hexstr), `"`)
		c, err := parseHexRGBA(hexstr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = c
	}
	return out, sc.Err()
}

func parseHexRGBA(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 8 {
		return color.RGBA{}, fmt.Errorf("need #RRGGBBAA, got %q", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: b[3]}, nil
}
