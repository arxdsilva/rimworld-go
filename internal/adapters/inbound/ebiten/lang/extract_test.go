package lang

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var update = flag.Bool("update", false, "rewrite assets/lang/en.json from the client's T/Tf calls")

// TestCatalogIsCurrent: assets/lang/en.json holds exactly the keys the
// client passes to T and Tf as literals (values = keys). It is the
// template translators copy; run with -update after adding text.
func TestCatalogIsCurrent(t *testing.T) {
	t.Parallel()
	keys := map[string]bool{}
	for _, f := range parseClient(t) {
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if k, ok := translateKey(f, call); ok && k != "" {
					keys[k] = true
				}
			}
			return true
		})
	}
	want := renderEnglish(t, keys)
	file := filepath.Join(repoRoot(t), "assets", "lang", EnglishCode+".json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v; run go test ./%s/lang -run TestCatalogIsCurrent -update", err, ebitenRel)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("assets/lang/en.json is stale (%d keys in the code); run go test ./%s/lang -run TestCatalogIsCurrent -update", len(keys), ebitenRel)
	}
}

// renderEnglish writes the English template: _meta first, then every key
// mapped to itself, sorted, one per line.
func renderEnglish(t *testing.T, keys map[string]bool) []byte {
	t.Helper()
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	slices.Sort(sorted)
	var b bytes.Buffer
	b.WriteString("{\n  \"" + metaKey + "\": {\"name\": \"English\"}")
	for _, k := range sorted {
		q := quote(t, k)
		b.WriteString(",\n  " + q + ": " + q)
	}
	b.WriteString("\n}\n")
	return b.Bytes()
}

// quote is s as a JSON string without HTML escaping.
func quote(t *testing.T, s string) string {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimRight(b.Bytes(), "\n"))
}

// TestExtractorFindsEveryForm: the call shapes the extractor reads.
func TestExtractorFindsEveryForm(t *testing.T) {
	src := `package x
import "example.com/` + ebitenRel + `/lang"
func f(tx tr, l lang.Tr) {
	_ = tx.T("Drop")
	_ = tx.Tf("%d kg", 3)
	_ = lang.Tf(l, "Hello %s", "you")
	_ = tx.T(name)
	_ = tx.Other("Nope")
}`
	f := parseOne(t, src)
	var got []string
	ast.Inspect(f.ast, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if k, ok := translateKey(f, call); ok {
				got = append(got, k)
			}
		}
		return true
	})
	if want := []string{"Drop", "%d kg", "Hello %s"}; !slices.Equal(got, want) {
		t.Fatalf("keys %q, want %q", got, want)
	}
	keys := map[string]bool{"b \"q\" <x>": true, "a": true}
	want := "{\n  \"_meta\": {\"name\": \"English\"},\n  \"a\": \"a\",\n  \"b \\\"q\\\" <x>\": \"b \\\"q\\\" <x>\"\n}\n"
	if got := string(renderEnglish(t, keys)); got != want {
		t.Fatalf("render:\n%s\nwant:\n%s", got, want)
	}
	c, err := parse(EnglishCode, []byte(want))
	if err != nil || c.Len() != 2 || c.Name() != "English" {
		t.Fatalf("the template does not load back: %v %d %q", err, c.Len(), c.Name())
	}
}
