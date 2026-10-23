package audiomanifest

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const assetsRoot = "../../assets"

// allowGlobs reads assets/metadata/orphan_allow.txt: `<path-or-glob>  # reason`,
// paths relative to assets/. A line without a reason is an error.
func allowGlobs(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(assetsRoot, "metadata", "orphan_allow.txt"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var globs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		glob, reason, _ := strings.Cut(line, "#")
		if strings.TrimSpace(reason) == "" {
			t.Errorf("orphan_allow.txt: %q has no '# reason'", line)
		}
		globs = append(globs, strings.TrimSpace(glob))
	}
	return globs
}

func allowed(globs []string, rel string) bool {
	for _, g := range globs {
		if ok, _ := path.Match(g, rel); ok {
			return true
		}
	}
	return false
}

// TestAudioFilesMatchManifest: every file under assets/generated/audio is
// listed in audio.json (or allowlisted with a reason) and every listed file
// exists.
func TestAudioFilesMatchManifest(t *testing.T) {
	raw, err := os.Open(filepath.Join(assetsRoot, filepath.FromSlash(Rel)))
	if err != nil {
		t.Skip("sound list not present:", err)
	}
	defer raw.Close()
	m, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, e := range m.Sounds {
		for _, f := range e.Files {
			listed[f] = true
			if _, err := os.Stat(filepath.Join(assetsRoot, filepath.FromSlash(Dir), filepath.FromSlash(f))); err != nil {
				t.Errorf("%s lists %s, which does not exist", e.ID, f)
			}
		}
	}
	globs := allowGlobs(t)
	var orphans []string
	root := filepath.Join(assetsRoot, filepath.FromSlash(Dir))
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if !listed[rel] && !allowed(globs, Dir+"/"+rel) {
			orphans = append(orphans, Dir+"/"+rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("%d sound files are not in audio.json (remove them or list them in assets/metadata/orphan_allow.txt with a reason), e.g.\n%s",
			len(orphans), strings.Join(orphans[:min(len(orphans), 20)], "\n"))
	}
}
