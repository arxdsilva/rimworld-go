package modloader

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"rimworld-go/internal/application/modding"
)

func write(t *testing.T, base, rel, data string) {
	t.Helper()
	p := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, roots ...string) []modding.Plugin {
	t.Helper()
	got, err := New(roots...).Plugins()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func paths(fs []modding.File) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Path)
	}
	return out
}

func hasProblem(p modding.Plugin, file, sub string) bool {
	for _, pr := range p.Problems {
		if pr.File == file && strings.Contains(pr.Msg, sub) {
			return true
		}
	}
	return false
}

const goodManifest = `{"id":"demo","name":"Demo","version":"1.0.0"}`

func TestValidPlugin(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/defs/z.json", `{"a":1}`)
	write(t, root, "demo/defs/sub/b.json", `[1,2]`)
	write(t, root, "demo/defs/a.json", `{}`)
	write(t, root, "demo/defs/readme.txt", `x`)
	write(t, root, "demo/patches/p.json", `[]`)
	write(t, root, "demo/lang/en.json", `{}`)
	write(t, root, "demo/lang/pt-BR.json", `{}`)
	write(t, root, "demo/textures/t.json", `{}`)
	got := load(t, root)
	if len(got) != 1 {
		t.Fatalf("plugins = %d", len(got))
	}
	p := got[0]
	if p.Manifest.ID != "demo" || p.Manifest.Version != "1.0.0" || len(p.Problems) != 0 {
		t.Fatalf("manifest/problems: %+v %v", p.Manifest, p.Problems)
	}
	if want := []string{"defs/a.json", "defs/sub/b.json", "defs/z.json"}; !reflect.DeepEqual(paths(p.Defs), want) {
		t.Fatalf("defs = %v", paths(p.Defs))
	}
	if !reflect.DeepEqual(paths(p.Patches), []string{"patches/p.json"}) ||
		!reflect.DeepEqual(paths(p.Lang), []string{"lang/en.json", "lang/pt-BR.json"}) {
		t.Fatalf("patches/lang = %v %v", paths(p.Patches), paths(p.Lang))
	}
	if m, ok := p.Defs[2].Data.(map[string]any); !ok || m["a"] != float64(1) {
		t.Fatalf("data = %#v", p.Defs[2].Data)
	}
}

func TestMissingRootSkipped(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	got := load(t, filepath.Join(root, "nope"), root)
	if len(got) != 1 {
		t.Fatalf("plugins = %d", len(got))
	}
}

func TestUnlistableRootErrors(t *testing.T) {
	root := t.TempDir()
	write(t, root, "file", "x")
	if _, err := New(filepath.Join(root, "file")).Plugins(); err == nil {
		t.Fatal("want error for a root that is a file")
	}
}

func TestMissingManifest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "bare/defs/a.json", `{}`)
	p := load(t, root)[0]
	if p.Manifest.ID != "bare" || p.Manifest.Name != "bare" || !hasProblem(p, "plugin.json", "missing") {
		t.Fatalf("%+v", p)
	}
	if p.Problems[0].Plugin != "bare" || len(p.Defs) != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestUnknownManifestField(t *testing.T) {
	root := t.TempDir()
	write(t, root, "x/plugin.json", `{"id":"x","colour":"red"}`)
	p := load(t, root)[0]
	if len(p.Problems) != 1 || p.Problems[0].Field != "colour" || !strings.Contains(p.Problems[0].Msg, "colour") {
		t.Fatalf("%v", p.Problems)
	}
	if p.Manifest.ID != "x" {
		t.Fatalf("id = %q", p.Manifest.ID)
	}
}

func TestBadJSONLineColumn(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/defs/bad.json", "{\n  \"a\": 1,\n  \"b\": ?\n}")
	p := load(t, root)[0]
	if len(p.Defs) != 0 || !hasProblem(p, "defs/bad.json", "line 3, column 8") {
		t.Fatalf("%v", p.Problems)
	}
}

func TestTrailingAndEmpty(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/defs/t.json", "{}\n{}")
	write(t, root, "demo/defs/e.json", "")
	write(t, root, "demo/defs/ok.json", "{}\n")
	write(t, root, "bad/plugin.json", goodManifest+" 1")
	got := load(t, root)
	var demo, bad modding.Plugin
	for _, p := range got {
		if p.Dir == filepath.Join(root, "demo") {
			demo = p
		} else {
			bad = p
		}
	}
	if !hasProblem(demo, "defs/t.json", "after the top-level value") || !hasProblem(demo, "defs/e.json", "empty") ||
		len(demo.Defs) != 1 {
		t.Fatalf("%v", demo.Problems)
	}
	if bad.Manifest.ID != "bad" || !hasProblem(bad, "plugin.json", "after the top-level value") {
		t.Fatalf("%+v", bad)
	}
}

func TestFileOverCap(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	big := `"` + strings.Repeat("a", maxDataFileBytes) + `"` // > cap
	write(t, root, "demo/defs/big.json", big)
	write(t, root, "demo/defs/ok.json", `1`)
	p := load(t, root)[0]
	if !hasProblem(p, "defs/big.json", "larger than") || !reflect.DeepEqual(paths(p.Defs), []string{"defs/ok.json"}) {
		t.Fatalf("%v", p.Problems)
	}
}

func TestTooManyFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	for i := 0; i < maxDataFiles+3; i++ {
		write(t, root, "demo/defs/f"+strings.Repeat("0", 5-len(itoa(i)))+itoa(i)+".json", `1`)
	}
	p := load(t, root)[0]
	if len(p.Defs) != maxDataFiles || len(p.Problems) != 1 || !strings.Contains(p.Problems[0].Msg, "too many") {
		t.Fatalf("defs=%d problems=%v", len(p.Defs), p.Problems)
	}
	if p.Defs[maxDataFiles-1].Path != "defs/f"+strings.Repeat("0", 5-len(itoa(maxDataFiles-1)))+itoa(maxDataFiles-1)+".json" {
		t.Fatalf("last = %s", p.Defs[maxDataFiles-1].Path)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

func TestLangFilter(t *testing.T) {
	root := t.TempDir()
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/lang/en.json", `{}`)
	write(t, root, "demo/lang/e.json", `{}`)
	write(t, root, "demo/lang/english.json", `{}`)
	write(t, root, "demo/lang/en-us.json", `{}`)
	write(t, root, "demo/lang/sub/pt.json", `{}`)
	p := load(t, root)[0]
	if !reflect.DeepEqual(paths(p.Lang), []string{"lang/en.json"}) {
		t.Fatalf("lang = %v", paths(p.Lang))
	}
	warns := 0
	for _, pr := range p.Problems {
		if pr.Warn && pr.Msg == "not a locale file" {
			warns++
		}
	}
	if warns != 3 || len(p.Problems) != 3 {
		t.Fatalf("%v", p.Problems)
	}
}

func TestDotFolderAndFilesSkipped(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".git/plugin.json", goodManifest)
	write(t, root, "stray.json", `{}`)
	write(t, root, "demo/plugin.json", goodManifest)
	if got := load(t, root); len(got) != 1 {
		t.Fatalf("plugins = %d", len(got))
	}
}

func TestSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "secret.json", `{"s":1}`)
	write(t, outside, "dir/x.json", `{}`)
	write(t, root, "demo/plugin.json", goodManifest)
	write(t, root, "demo/defs/in.json", `{}`)
	if err := os.Symlink(filepath.Join(outside, "secret.json"), filepath.Join(root, "demo/defs/evil.json")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "demo/defs/evildir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("in.json", filepath.Join(root, "demo/defs/alias.json")); err != nil {
		t.Fatal(err)
	}
	p := load(t, root)[0]
	if !hasProblem(p, "defs/evil.json", "outside the plugin folder") || !hasProblem(p, "defs/evildir", "outside the plugin folder") {
		t.Fatalf("%v", p.Problems)
	}
	if !reflect.DeepEqual(paths(p.Defs), []string{"defs/alias.json", "defs/in.json"}) {
		t.Fatalf("defs = %v", paths(p.Defs))
	}
}

func TestSymlinkedDefsDirEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "x.json", `{}`)
	write(t, root, "demo/plugin.json", goodManifest)
	if err := os.Symlink(outside, filepath.Join(root, "demo/defs")); err != nil {
		t.Skip(err)
	}
	p := load(t, root)[0]
	if len(p.Defs) != 0 || !hasProblem(p, "defs", "outside the plugin folder") {
		t.Fatalf("%v", p.Problems)
	}
}

func TestOrderIndependentOfCreation(t *testing.T) {
	names := []string{"b", "a", "c"}
	build := func(order []string) []modding.Plugin {
		root := t.TempDir()
		for _, n := range order {
			write(t, root, n+"/plugin.json", `{"id":"`+n+`"}`)
			write(t, root, n+"/defs/2.json", `1`)
			write(t, root, n+"/defs/1.json", `2`)
		}
		got := load(t, root)
		for i := range got {
			got[i].Dir = filepath.Base(got[i].Dir)
		}
		return got
	}
	a := build(names)
	b := build([]string{"c", "a", "b"})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("differ:\n%+v\n%+v", a, b)
	}
	if a[0].Manifest.ID != "a" || a[2].Manifest.ID != "c" {
		t.Fatalf("not sorted: %+v", a)
	}
}

func TestDuplicateIDAcrossRoots(t *testing.T) {
	r1, r2 := t.TempDir(), t.TempDir()
	write(t, r1, "one/plugin.json", `{"id":"dup"}`)
	write(t, r2, "two/plugin.json", `{"id":"dup"}`)
	got := load(t, r1, r2)
	if len(got) != 2 || got[0].Manifest.ID != "dup" || got[1].Manifest.ID != "dup" {
		t.Fatalf("%+v", got)
	}
	if got[0].Dir == got[1].Dir {
		t.Fatal("same dir")
	}
}
