package art

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Benchmarks of the hand-written YAML-subset parsers over the real assets,
// so a change of parser (or a decision to keep this one) rests on numbers.
// The file set is exactly what Index(root, false) (what Load uses) feeds
// ParseMeta: every family directory's *.yaml except the atlas index, the
// menu scene and packed families, plus the metadata packed in the atlas
// indexes. Files are read once before the timer: no disk in the numbers.

// parseBenchRoot is the assets root, or skips the benchmark.
func parseBenchRoot(b *testing.B) string {
	b.Helper()
	root, err := Locate()
	if err != nil {
		b.Skip("assets not present:", err)
	}
	return root
}

// metaSources reads the individual metadata files Index parses with ParseMeta.
func metaSources(b *testing.B, root string) []string {
	b.Helper()
	metaRoot := filepath.Join(root, MetadataRel)
	families, err := os.ReadDir(metaRoot)
	if err != nil {
		b.Skip("no metadata:", err)
	}
	var out []string
	for _, fam := range families {
		if !fam.IsDir() || fam.Name() == menuMetaFamily || fam.Name() == AtlasMetaDir {
			continue
		}
		if _, err := os.Stat(filepath.Join(metaRoot, AtlasMetaDir, fam.Name()+".yaml")); err == nil {
			continue // packed: served from the atlas index
		}
		files, err := os.ReadDir(filepath.Join(metaRoot, fam.Name()))
		if err != nil {
			b.Fatal(err)
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".yaml") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(metaRoot, fam.Name(), f.Name()))
			if err != nil {
				b.Fatal(err)
			}
			out = append(out, string(raw))
		}
	}
	return out
}

// atlasSources reads every atlas index (metadata/atlas/*.yaml).
func atlasSources(b *testing.B, root string) []string {
	b.Helper()
	packed, _ := filepath.Glob(filepath.Join(root, MetadataRel, AtlasMetaDir, "*.yaml"))
	var out []string
	for _, p := range packed {
		raw, err := os.ReadFile(p)
		if err != nil {
			b.Fatal(err)
		}
		out = append(out, string(raw))
	}
	return out
}

func BenchmarkParseMetaAll(b *testing.B) {
	srcs := metaSources(b, parseBenchRoot(b))
	if len(srcs) == 0 {
		b.Skip("no metadata files")
	}
	var bytes int
	for _, s := range srcs {
		bytes += len(s)
	}
	b.SetBytes(int64(bytes))
	b.ReportMetric(float64(len(srcs)), "files")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range srcs {
			if _, err := ParseMeta(s); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkParseAtlasAll parses the atlas indexes and then each packed
// sprite's embedded metadata with ParseMeta, as loadAtlas does.
func BenchmarkParseAtlasAll(b *testing.B) {
	srcs := atlasSources(b, parseBenchRoot(b))
	if len(srcs) == 0 {
		b.Skip("no atlas indexes")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range srcs {
			_, _, entries, err := ParseAtlas(s)
			if err != nil {
				b.Fatal(err)
			}
			for _, e := range entries {
				if _, err := ParseMeta(e.Meta); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}

// BenchmarkParseTints parses palettes/base.yaml.
func BenchmarkParseTints(b *testing.B) {
	raw, err := os.ReadFile(filepath.Join(parseBenchRoot(b), "palettes", "base.yaml"))
	if err != nil {
		b.Skip(err)
	}
	src := string(raw)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseTints(src); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkIndex is the whole boot-time index (Load minus the FreeCPU flag):
// base palette, atlas indexes with their page headers, every unpacked
// metadata file, all read from disk (warm OS cache). Index is sequential.
func BenchmarkIndex(b *testing.B) {
	root := parseBenchRoot(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Index(root, false); err != nil {
			b.Fatal(err)
		}
	}
}
