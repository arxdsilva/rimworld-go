package lang

import (
	"bytes"
	"encoding/json"
	"flag"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var update = flag.Bool("update", false, "rewrite assets/lang/en.json from the client's T/Tf calls and the game's posts")

// TestCatalogIsCurrent: assets/lang/en.json holds exactly the keys the
// client passes to T and Tf as literals, and the simulation's message and
// letter keys (simKeys; values = keys). It is the
// template translators copy; run with -update after adding text.
func TestCatalogIsCurrent(t *testing.T) {
	t.Parallel()
	keys := catalogKeys(parseClient(t))
	sim, built, finished := simKeys(parseTree(t, gameRel), module(t, repoRoot(t))+"/"+gameRel)
	for k := range sim {
		keys[k] = true
	}
	// Formats built at run time and finished text reach the catalog as a
	// Plain key no template holds, so they stay English (-v lists them).
	t.Logf("%d simulation keys; %d formats built at run time, %d finished-text posts", len(sim), len(built), len(finished))
	for _, u := range built {
		t.Log("  built:    " + u)
	}
	for _, u := range finished {
		t.Log("  finished: " + u)
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

// catalogKeys is every key the client translates: the literal a T/Tf call
// carries, or the string constant of its own package it names (T(pausedLabel)).
func catalogKeys(files []*srcFile) map[string]bool {
	consts := stringConsts(files)
	keys := map[string]bool{}
	for _, f := range files {
		ast.Inspect(f.ast, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			k, ok := translateKey(f, call)
			if id, isID := translateArg(f, call).(*ast.Ident); !ok && isID {
				k, ok = consts[f.pkg][id.Name]
			}
			if ok && k != "" {
				keys[k] = true
			}
			return true
		})
	}
	return keys
}

// stringConsts is each package's string constants: pkg -> name -> value.
func stringConsts(files []*srcFile) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, f := range files {
		for _, d := range f.ast.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, spec := range g.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if v, ok := strLit(vs.Values[i]); ok {
						if out[f.pkg] == nil {
							out[f.pkg] = map[string]string{}
						}
						out[f.pkg][n.Name] = v
					}
				}
			}
		}
	}
	return out
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
	cf := parseOne(t, "package x\nconst paused = \"Paused\"\nfunc g(tx tr) { _ = tx.T(paused); _ = tx.T(other) }")
	if got := catalogKeys([]*srcFile{cf}); len(got) != 1 || !got["Paused"] {
		t.Fatalf("constant keys %v, want Paused", got)
	}
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
