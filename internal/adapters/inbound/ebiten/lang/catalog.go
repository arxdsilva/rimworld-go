package lang

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// EnglishCode is the source language's code; it needs no file.
const EnglishCode = "en"

// metaKey holds the catalog's own facts ({"name": "Français"}), not a
// translation.
const metaKey = "_meta"

// codeRE is a catalog code: a lower-case language, then optional
// region or script subtags ("pt-BR", "zh-Hans"). It also keeps a code from
// naming a path outside the catalog folder.
var codeRE = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// Catalog is one language's translations, keyed by the English source.
// The zero Catalog is English.
type Catalog struct {
	code, name string
	m          map[string]string
}

// T returns src's translation, or src when the catalog lacks it.
func (c Catalog) T(src string) string {
	if v, ok := c.m[src]; ok {
		return v
	}
	return src
}

// Code is the catalog's code ("en" for the zero Catalog).
func (c Catalog) Code() string {
	if c.code == "" {
		return EnglishCode
	}
	return c.code
}

// Name is the language's own name ("English" for the zero Catalog).
func (c Catalog) Name() string {
	if c.name == "" {
		return "English"
	}
	return c.name
}

// Len is how many source strings the catalog translates.
func (c Catalog) Len() int { return len(c.m) }

// Choice is one language the Language option can offer.
type Choice struct {
	Code, Name string
}

type meta struct {
	Name string `json:"name"`
}

// Load reads dir/<code>.json. English loads without a file, and its
// entries are never applied (keys are the source, so its text is the
// source byte for byte). An empty translation, or one whose fmt verbs
// differ from its source's, is dropped so that entry shows the source.
func Load(dir, code string) (Catalog, error) {
	if !codeRE.MatchString(code) {
		return Catalog{}, fmt.Errorf("lang: bad catalog code %q", code)
	}
	b, err := os.ReadFile(filepath.Join(dir, code+".json"))
	if errors.Is(err, fs.ErrNotExist) && code == EnglishCode {
		return Catalog{}, nil
	}
	if err != nil {
		return Catalog{}, fmt.Errorf("lang: %w", err)
	}
	c, err := parse(code, b)
	if err != nil {
		return Catalog{}, err
	}
	if code == EnglishCode {
		c.m = nil
	}
	return c, nil
}

// parse decodes one catalog file.
func parse(code string, b []byte) (Catalog, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return Catalog{}, fmt.Errorf("lang: %s.json: %w", code, err)
	}
	c := Catalog{code: code, name: code, m: make(map[string]string, len(raw))}
	for k, v := range raw {
		if k == metaKey {
			var m meta
			if err := json.Unmarshal(v, &m); err != nil {
				return Catalog{}, fmt.Errorf("lang: %s.json %s: %w", code, metaKey, err)
			}
			if m.Name != "" {
				c.name = m.Name
			}
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return Catalog{}, fmt.Errorf("lang: %s.json %q: %w", code, k, err)
		}
		if s != "" && sameVerbs(k, s) {
			c.m[k] = s
		}
	}
	return c, nil
}

// Codes lists the catalogs in dir, English first and the rest by code.
// English is listed even without a file; an unreadable catalog is left
// out (Load reports why).
func Codes(dir string) []Choice {
	out := []Choice{{Code: EnglishCode, Name: "English"}}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		code, ok := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !ok || !codeRE.MatchString(code) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		c, err := parse(code, b)
		if err != nil {
			continue
		}
		if code == EnglishCode {
			out[0].Name = c.Name()
			continue
		}
		out = append(out, Choice{Code: code, Name: c.Name()})
	}
	slices.SortFunc(out[1:], func(a, b Choice) int { return strings.Compare(a.Code, b.Code) })
	return out
}

// verbs lists s's fmt verbs (letters, explicit indexes kept, "%%"
// skipped) in order.
func verbs(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(s) && strings.IndexByte("+-# 0123456789.*[]", s[j]) >= 0 {
			j++
		}
		if j >= len(s) {
			break
		}
		if s[j] != '%' {
			out = append(out, s[i+1:j+1])
		}
		i = j
	}
	return out
}

// sameVerbs reports whether two strings carry the same fmt verbs, in any
// order (a translation may reorder with explicit indexes).
func sameVerbs(a, b string) bool {
	va, vb := verbLetters(a), verbLetters(b)
	return slices.Equal(va, vb)
}

func verbLetters(s string) []string {
	v := verbs(s)
	for i, x := range v {
		v[i] = x[len(x)-1:]
	}
	slices.Sort(v)
	return v
}
