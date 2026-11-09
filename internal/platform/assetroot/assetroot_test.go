package assetroot

import (
	"os"
	"path/filepath"
	"testing"
)

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets", "metadata", "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "metadata", "audio", "audio.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bin", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFindUpFromBelow(t *testing.T) {
	t.Parallel()
	root := tree(t)
	p, ok := findUp(filepath.Join(root, "bin", "deep"), filepath.Join("assets", "metadata"), true)
	if !ok || p != filepath.Join(root, "assets", "metadata") {
		t.Fatalf("findUp = %q, %v", p, ok)
	}
	if _, ok := findUp(filepath.Join(root, "bin"), filepath.Join("assets", "metadata"), false); ok {
		t.Fatal("a directory must not match a file search")
	}
	if _, ok := findUp(filepath.Join(root, "bin"), "nope", true); ok {
		t.Fatal("found a missing path")
	}
}

func TestFindParentReturnsRoot(t *testing.T) {
	root := tree(t)
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(filepath.Join(root, "bin", "deep")); err != nil {
		t.Fatal(err)
	}
	got, err := FindParent("assets", filepath.Join("metadata", "audio", "audio.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "assets"))
	if g, _ := filepath.EvalSymlinks(got); g != want {
		t.Fatalf("FindParent = %q, want %q", got, want)
	}
	if _, err := Find("definitely-not-here", true); err == nil {
		t.Fatal("expected an error for a missing path")
	}
}

// mkfile writes an empty file at root/rel, making its folders.
func mkfile(t *testing.T, root, rel string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// firstHit is the path the search order finds rel at, as Find does.
func firstHit(starts []string, rel string, dir bool) string {
	for _, s := range starts {
		if p, ok := findUp(s, rel, dir); ok {
			return p
		}
	}
	return ""
}

// A macOS bundle: X.app/Contents/MacOS/bin finds X.app/Contents/Resources/assets,
// before anything the executable's own folder walks up to.
func TestMacBundleFindsItsResources(t *testing.T) {
	t.Parallel()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	exe := mkfile(t, root, "Applications/X.app/Contents/MacOS/bin")
	mkfile(t, root, "Applications/X.app/Contents/Resources/assets/lang/en.json")
	mkfile(t, root, "Applications/assets/lang/en.json") // a decoy beside the bundle
	starts := startsFrom("/", exe)
	want := filepath.Join(root, "Applications", "X.app", "Contents", "Resources", "assets", "lang")
	if got := firstHit(starts, filepath.Join("assets", "lang"), true); got != want {
		t.Fatalf("found %q, want %q (starts %q)", got, want, starts)
	}
	if got := startsFrom("", filepath.Join(root, "plain", "bin")); len(got) != 1 || got[0] != filepath.Join(root, "plain") {
		t.Fatalf("a plain folder gets no Resources start: %q", got)
	}
}

// A packaged Linux install: /usr/bin/rimworld-go is a symlink into
// /opt/rimworld-go, where the assets are.
func TestSymlinkedExecutableFindsTheRealFolder(t *testing.T) {
	t.Parallel()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	real := mkfile(t, root, "opt/rimworld-go/rimworld-go")
	mkfile(t, root, "opt/rimworld-go/assets/metadata/audio/audio.json")
	link := filepath.Join(root, "usr", "bin", "rimworld-go")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	starts := startsFrom(filepath.Join(root, "home"), link)
	want := filepath.Join(root, "opt", "rimworld-go", "assets", "metadata")
	if got := firstHit(starts, filepath.Join("assets", "metadata"), true); got != want {
		t.Fatalf("found %q, want %q (starts %q)", got, want, starts)
	}
}
