package ebiten

import (
	"bufio"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Orphan check (PLAY-010): every generated sprite and its metadata must be
// reachable from Go code, or be listed with a reason in
// assets/metadata/orphan_allow.txt. The renderer builds keys dynamically
// ("tree_"+species, pawnKey(...), ...), so the check works on key *words*: a
// sprite name is split on '_', the variant, frame and facing parts are
// dropped, and every remaining word must occur in a string literal of the
// non-test Go sources (internal/, cmd/). A species, building or item whose
// words no code mentions is dead art. A word can be satisfied by an unrelated
// literal, so this under-reports; it never reports a sprite the code can ask for.

const assetsDir = "../../../../assets"

// spriteDirs maps metadata/<family> to the directory holding its PNGs.
var spriteDirs = map[string]string{
	"ui_icon": "generated/ui/icons", "ui_menu": "generated/ui/menu", "ui_portrait": "generated/ui/portraits",
}

func pngDir(family string) string {
	if d, ok := spriteDirs[family]; ok {
		return d
	}
	return "generated/world/" + family
}

var (
	wordSplit  = regexp.MustCompile(`[^a-z0-9]+`)
	genericTok = regexp.MustCompile(`^(v?\d+|north|south|east|west|n|mask|c[a-z])$`)
	trailDigit = regexp.MustCompile(`\d+$`)
	imageLine  = regexp.MustCompile(`(?m)^image:\s*(\S+)`)
)

// goLiteralWords is every lower-case word in a string literal of the
// production Go sources.
func goLiteralWords(t *testing.T) map[string]bool {
	t.Helper()
	words := map[string]bool{}
	repo := filepath.Join(assetsDir, "..")
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(repo, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fset := token.NewFileSet()
			file := fset.AddFile(p, -1, len(src))
			var s scanner.Scanner
			s.Init(file, src, nil, 0)
			for {
				_, tok, lit := s.Scan()
				if tok == token.EOF {
					return nil
				}
				if tok != token.STRING {
					continue
				}
				if u, err := strconv.Unquote(lit); err == nil {
					lit = u
				}
				for _, w := range wordSplit.Split(strings.ToLower(lit), -1) {
					if w != "" {
						words[w] = true
						words[trailDigit.ReplaceAllString(w, "")] = true // "flinch1" also covers flinch2
					}
				}
			}
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return words
}

// stemWords are the words of a sprite name that identify what it depicts.
func stemWords(name string) []string {
	var out []string
	for _, tok := range strings.Split(name, "_") {
		if genericTok.MatchString(tok) {
			continue
		}
		if w := trailDigit.ReplaceAllString(tok, ""); w != "" { // walk01, graze2
			out = append(out, w)
		}
	}
	return out
}

func isCompanion(name string) bool {
	return strings.HasSuffix(name, "_n.png") || strings.HasSuffix(name, "_mask.png")
}

// orphanAllow reads the allowlist; globs are relative to assets/.
func orphanAllow(t *testing.T) (globs []string, matched map[string]bool) {
	t.Helper()
	matched = map[string]bool{}
	f, err := os.Open(filepath.Join(assetsDir, "metadata", "orphan_allow.txt"))
	if err != nil {
		return nil, matched
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		g, reason, _ := strings.Cut(line, "#")
		if strings.TrimSpace(reason) == "" {
			t.Errorf("orphan_allow.txt: %q has no '# reason'", line)
		}
		globs = append(globs, strings.TrimSpace(g))
	}
	return globs, matched
}

// orphanReport lists the paths (relative to assets/) of sprites and
// metadata that no code reaches, ignoring the allowlist.
func orphanReport(words map[string]bool, root string) (orphans []string, all []string, err error) {
	fams, err := os.ReadDir(filepath.Join(root, "metadata"))
	if err != nil {
		return nil, nil, err
	}
	for _, fam := range fams {
		if !fam.IsDir() || fam.Name() == "atlas" || fam.Name() == "audio" {
			continue
		}
		family := fam.Name()
		haveMeta := map[string]bool{}
		metas, _ := os.ReadDir(filepath.Join(root, "metadata", family))
		for _, m := range metas {
			if !strings.HasSuffix(m.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(m.Name(), ".yaml")
			haveMeta[name] = true
			rel := "metadata/" + family + "/" + m.Name()
			raw, _ := os.ReadFile(filepath.Join(root, rel))
			img := ""
			if mm := imageLine.FindSubmatch(raw); mm != nil {
				img = string(mm[1])
			}
			all = append(all, rel)
			if img != "" {
				if _, err := os.Stat(filepath.Join(root, img)); err != nil {
					orphans = append(orphans, rel) // metadata whose PNG is gone
					continue
				}
			}
			if !reachable(words, name) {
				orphans = append(orphans, rel)
				if img != "" {
					orphans = append(orphans, img)
				}
			}
		}
		dir := pngDir(family)
		pngs, _ := os.ReadDir(filepath.Join(root, dir))
		for _, p := range pngs {
			if p.IsDir() || !strings.HasSuffix(p.Name(), ".png") || isCompanion(p.Name()) {
				continue
			}
			name := strings.TrimSuffix(p.Name(), ".png")
			all = append(all, dir+"/"+p.Name())
			if !haveMeta[name] && !metaMentions(root, dir+"/"+p.Name()) {
				orphans = append(orphans, dir+"/"+p.Name()) // PNG without metadata
			}
		}
	}
	sort.Strings(orphans)
	return dedupe(orphans), all, nil
}

// metaMentions reports whether any metadata yaml names the PNG (a menu
// diorama lists its glow layers that way).
func metaMentions(root, rel string) bool {
	hit := false
	_ = filepath.WalkDir(filepath.Join(root, "metadata"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !hit && !d.IsDir() && strings.HasSuffix(p, ".yaml") {
			if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), rel) {
				hit = true
			}
		}
		return nil
	})
	return hit
}

func reachable(words map[string]bool, name string) bool {
	for _, w := range stemWords(name) {
		if !words[w] {
			return false
		}
	}
	return true
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func TestNoOrphanSprites(t *testing.T) {
	if _, err := os.Stat(filepath.Join(assetsDir, "metadata")); err != nil {
		t.Skip("assets not present")
	}
	words := goLiteralWords(t)
	orphans, all, err := orphanReport(words, assetsDir)
	if err != nil {
		t.Fatal(err)
	}
	globs, matched := orphanAllow(t)
	var bad []string
	for _, o := range orphans {
		ok := false
		for _, g := range globs {
			if m, _ := path.Match(g, o); m {
				matched[g] = true
				ok = true
			}
		}
		if !ok {
			bad = append(bad, o)
		}
	}
	if len(bad) > 0 {
		t.Errorf("%d generated files nothing references (delete them with their recipe, or list them in assets/metadata/orphan_allow.txt as `<glob>  # reason`), e.g.\n%s",
			len(bad), strings.Join(bad[:min(len(bad), 25)], "\n"))
	}
	// A stale allowlist entry hides nothing: it must still match a file.
	for _, g := range globs {
		if matched[g] {
			continue
		}
		for _, a := range all {
			if m, _ := path.Match(g, a); m {
				matched[g] = true
				break
			}
		}
		if !matched[g] {
			t.Errorf("orphan_allow.txt: %q matches no file; remove it", g)
		}
	}
}

// TestOrphanReportFakeTree: one used sprite, one orphan, one allowlisted.
func TestOrphanReportFakeTree(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"bush_v0", "yucca_v0"} {
		write("generated/world/plant/"+n+".png", "x")
		write("generated/world/plant/"+n+"_n.png", "x")
		write("metadata/plant/"+n+".yaml", "image: generated/world/plant/"+n+".png\n")
	}
	write("generated/world/plant/stray.png", "x") // PNG without metadata
	orphans, _, err := orphanReport(map[string]bool{"bush": true, "stray": true}, root)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(orphans, ",")
	want := "generated/world/plant/stray.png,generated/world/plant/yucca_v0.png,metadata/plant/yucca_v0.yaml"
	if got != want {
		t.Errorf("orphans = %s\nwant      %s", got, want)
	}
}

func TestStemWords(t *testing.T) {
	cases := map[string]string{
		"tree_oak_v2": "tree oak", "alpaca_east_walk03": "alpaca walk", "grass_short_v0": "grass short",
		"wall_wood_15_cf": "wall wood",
	}
	for in, want := range cases {
		if got := strings.Join(stemWords(in), " "); got != want {
			t.Errorf("stemWords(%q) = %q, want %q", in, got, want)
		}
	}
}
