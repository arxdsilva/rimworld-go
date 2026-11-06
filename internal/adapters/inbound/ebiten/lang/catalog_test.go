package lang

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeCatalog(t *testing.T, dir, code, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, code+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTranslatesAndFallsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCatalog(t, dir, "fr", `{
		"_meta": {"name": "Français"},
		"Drop": "Lâcher",
		"%d kg": "%d kilos",
		"%s ate %s": "%[2]s mangé par %[1]s",
		"Empty": "",
		"Lost verb %d": "Perdu"
	}`)
	c, err := Load(dir, "fr")
	if err != nil {
		t.Fatal(err)
	}
	if c.Code() != "fr" || c.Name() != "Français" {
		t.Fatalf("code %q name %q", c.Code(), c.Name())
	}
	l := &Live{}
	l.Set(c)
	for src, want := range map[string]string{
		"Drop":         "Lâcher",
		"Eat":          "Eat",          // missing key: the source
		"Empty":        "Empty",        // blank translation: the source
		"Lost verb %d": "Lost verb %d", // verbs differ: dropped
	} {
		if got := l.T(src); got != want {
			t.Errorf("T(%q) = %q, want %q", src, got, want)
		}
	}
	if got := l.Tf("%d kg", 3); got != "3 kilos" {
		t.Errorf("Tf = %q", got)
	}
	if got := l.Tf("%s ate %s", "Ann", "bread"); got != "bread mangé par Ann" {
		t.Errorf("reordered Tf = %q", got)
	}
}

func TestEnglishLoadsWithoutAFileAndIsTheSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := Load(dir, EnglishCode)
	if err != nil || c.Code() != "en" || c.Name() != "English" || c.T("Drop") != "Drop" {
		t.Fatalf("English without a file: %v %q %q", err, c.Code(), c.Name())
	}
	// A hand-edited en.json never changes English text: keys are the source.
	writeCatalog(t, dir, "en", `{"_meta": {"name": "English"}, "Drop": "Release"}`)
	c, err = Load(dir, EnglishCode)
	if err != nil || c.T("Drop") != "Drop" || c.Len() != 0 {
		t.Fatalf("English applied its file: %v %q", err, c.T("Drop"))
	}
	var zero Catalog
	if zero.T("Drop") != "Drop" || zero.Code() != "en" || zero.Name() != "English" {
		t.Fatal("the zero Catalog is not English")
	}
}

func TestLoadRejects(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeCatalog(t, dir, "de", `{"Drop": 3}`)
	writeCatalog(t, dir, "es", `not json`)
	for _, code := range []string{"de", "es", "it", "../en", "EN", ""} {
		if _, err := Load(dir, code); err == nil {
			t.Errorf("Load(%q) succeeded", code)
		}
	}
}

func TestCodesListsCatalogsEnglishFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := Codes(filepath.Join(dir, "missing")); !slices.Equal(got, []Choice{{"en", "English"}}) {
		t.Fatalf("no folder: %v", got)
	}
	writeCatalog(t, dir, "pt-BR", `{"_meta": {"name": "Português (Brasil)"}}`)
	writeCatalog(t, dir, "de", `{"_meta": {"name": "Deutsch"}, "Drop": "Fallen lassen"}`)
	writeCatalog(t, dir, "fr", `{}`)
	writeCatalog(t, dir, "es", `broken`)
	writeCatalog(t, dir, "Bad_Code", `{}`)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want := []Choice{{"en", "English"}, {"de", "Deutsch"}, {"fr", "fr"}, {"pt-BR", "Português (Brasil)"}}
	if got := Codes(dir); !slices.Equal(got, want) {
		t.Fatalf("Codes = %v, want %v", got, want)
	}
}

func TestVerbs(t *testing.T) {
	t.Parallel()
	got := verbs("%d%% of %-5.1f and %[2]s, %v")
	if want := []string{"d", "-5.1f", "[2]s", "v"}; !slices.Equal(got, want) {
		t.Fatalf("verbs = %q, want %q", got, want)
	}
	if !sameVerbs("%s ate %d", "%[2]d: %[1]s") || sameVerbs("%d", "%s") || sameVerbs("%d", "") {
		t.Fatal("sameVerbs")
	}
}

// TestShippedCatalogsLoad: every file in assets/lang loads, and none
// carries a translation the loader would drop for its verbs.
func TestShippedCatalogsLoad(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(repoRoot(t), "assets", "lang")
	for _, ch := range Codes(dir) {
		if _, err := Load(dir, ch.Code); err != nil {
			t.Errorf("%s: %v", ch.Code, err)
		}
	}
	for code, m := range shippedCatalogs(t, dir) {
		for k, v := range m {
			if v != "" && !sameVerbs(k, v) {
				t.Errorf("%s.json %q: verbs %q differ from %q", code, k, verbs(v), verbs(k))
			}
		}
	}
}
