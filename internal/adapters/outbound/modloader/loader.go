package modloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"rimworld-go/internal/application/modding"
)

const (
	maxManifestBytes = 64 << 10
	maxDataFileBytes = 4 << 20
	maxDataFiles     = 2000
	maxPluginBytes   = 32 << 20 // all of a plugin's data files together
	maxDepth         = 16
)

var localeRE = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

// Loader reads the plugins under its roots. It implements modding.Source.
type Loader struct {
	roots []string
}

// New makes a Loader over roots (missing roots are skipped).
func New(roots ...string) *Loader { return &Loader{roots: roots} }

// Plugins lists every plugin folder under the roots, sorted by id then
// directory. A folder whose manifest cannot be read is still listed, with
// its problems, under its folder name. Every read goes through an os.Root,
// so a path leaving the plugin folder is refused and reported.
func (l *Loader) Plugins() ([]modding.Plugin, error) {
	var out []modding.Plugin
	for _, root := range l.roots {
		entries, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("modloader: list %s: %w", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			out = append(out, readPlugin(filepath.Join(root, e.Name()), e.Name()))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Manifest.ID != out[j].Manifest.ID {
			return out[i].Manifest.ID < out[j].Manifest.ID
		}
		return out[i].Dir < out[j].Dir
	})
	return out, nil
}

type reader struct {
	root *os.Root
	pl   *modding.Plugin
	name string // folder name, until the manifest gives an id
	size int    // data bytes read so far (maxPluginBytes)
	full bool   // the plugin's data cap was reached: the rest is skipped
}

func (r *reader) problem(file, field, msg string, warn bool) {
	r.pl.Problems = append(r.pl.Problems, modding.Problem{
		Plugin: r.pl.Manifest.ID, File: file, Field: field, Msg: msg, Warn: warn})
}

func readPlugin(dir, name string) modding.Plugin {
	pl := modding.Plugin{Dir: dir, Manifest: modding.Manifest{ID: name, Name: name}}
	r := &reader{pl: &pl, name: name}
	root, err := os.OpenRoot(dir)
	if err != nil {
		r.problem("", "", "cannot open plugin folder: "+err.Error(), false)
		return pl
	}
	defer root.Close()
	r.root = root

	r.manifest()
	defs := r.collect("defs")
	patches := r.collect("patches")
	lang := r.lang()
	total := len(defs) + len(patches) + len(lang)
	if total > maxDataFiles {
		r.problem("", "", fmt.Sprintf("too many data files (%d, at most %d): the rest are skipped", total, maxDataFiles), false)
		budget := maxDataFiles
		defs, budget = trim(defs, budget)
		patches, budget = trim(patches, budget)
		lang, _ = trim(lang, budget)
	}
	pl.Defs = r.decodeAll(defs)
	pl.Patches = r.decodeAll(patches)
	pl.Lang = r.decodeAll(lang)
	return pl
}

func trim(s []string, n int) ([]string, int) {
	if len(s) > n {
		s = s[:n]
	}
	return s, n - len(s)
}

func (r *reader) manifest() {
	const file = "plugin.json"
	b, ok := r.read(file, maxManifestBytes)
	if !ok {
		return
	}
	var m modding.Manifest
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	err := dec.Decode(&m)
	if err == nil {
		err = trailing(dec)
	}
	if err != nil {
		field, msg := describe(b, err, dec)
		r.problem(file, field, msg, false)
		return
	}
	r.pl.Manifest = m
}

// read opens one file through the root and reads at most limit bytes.
func (r *reader) read(file string, limit int) ([]byte, bool) {
	f, err := r.root.Open(file)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist) && file == "plugin.json":
			r.problem(file, "", "missing plugin.json", false)
		case isEscape(err):
			r.problem(file, "", "points outside the plugin folder", false)
		default:
			r.problem(file, "", "cannot open: "+err.Error(), false)
		}
		return nil, false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		r.problem(file, "", "cannot read: "+err.Error(), false)
		return nil, false
	}
	if len(b) > limit {
		r.problem(file, "", fmt.Sprintf("file is larger than %d bytes", limit), false)
		return nil, false
	}
	return b, true
}

func isEscape(err error) bool {
	return err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) &&
		strings.Contains(err.Error(), "escapes")
}

// collect lists the .json files under dir, recursively, sorted.
func (r *reader) collect(dir string) []string {
	var out []string
	r.walk(dir, 0, &out, true)
	sort.Strings(out)
	return out
}

func (r *reader) walk(dir string, depth int, out *[]string, top bool) {
	if depth > maxDepth {
		r.problem(dir, "", "folders nested too deeply or looping", false)
		return
	}
	entries, err := fs.ReadDir(r.root.FS(), dir)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist) && top:
		case isEscape(err):
			r.problem(dir, "", "points outside the plugin folder", false)
		default:
			r.problem(dir, "", "cannot list: "+err.Error(), false)
		}
		return
	}
	for _, e := range entries {
		p := path.Join(dir, e.Name())
		isDir := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			st, err := r.root.Stat(p)
			if err != nil {
				if isEscape(err) {
					r.problem(p, "", "points outside the plugin folder", false)
				} else if !errors.Is(err, fs.ErrNotExist) {
					r.problem(p, "", "cannot stat: "+err.Error(), false)
				} else {
					r.problem(p, "", "dangling link", false)
				}
				continue
			}
			isDir = st.IsDir()
		}
		switch {
		case isDir:
			r.walk(p, depth+1, out, false)
		case strings.HasSuffix(e.Name(), ".json"):
			*out = append(*out, p)
		}
	}
}

func (r *reader) lang() []string {
	entries, err := fs.ReadDir(r.root.FS(), "lang")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			r.problem("lang", "", "cannot list: "+err.Error(), false)
		}
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := "lang/" + e.Name()
		if !localeRE.MatchString(strings.TrimSuffix(e.Name(), ".json")) {
			r.problem(p, "", "not a locale file", true)
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (r *reader) decodeAll(paths []string) []modding.File {
	var files []modding.File
	for _, p := range paths {
		if r.full {
			break
		}
		b, ok := r.read(p, maxDataFileBytes)
		if !ok {
			continue
		}
		if r.size += len(b); r.size > maxPluginBytes {
			r.full = true
			r.problem(p, "", fmt.Sprintf("the plugin's data passes %d MiB: this file and the rest are skipped", maxPluginBytes>>20), false)
			break
		}
		var v any
		dec := json.NewDecoder(bytes.NewReader(b))
		err := dec.Decode(&v)
		if err == nil {
			err = trailing(dec)
		}
		if err != nil {
			_, msg := describe(b, err, dec)
			r.problem(p, "", msg, false)
			continue
		}
		files = append(files, modding.File{Path: p, Data: v})
	}
	return files
}

var errTrailing = errors.New("unexpected data after the top-level value")

func trailing(dec *json.Decoder) error {
	if _, err := dec.Token(); err == io.EOF {
		return nil
	}
	return errTrailing
}

// describe turns a decode error into a field and a message with line:col.
func describe(b []byte, err error, dec *json.Decoder) (field, msg string) {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.Is(err, io.EOF):
		return "", "file is empty"
	case errors.As(err, &syn):
		return "", fmt.Sprintf("invalid JSON at %s: %v", lineCol(b, syn.Offset), err)
	case errors.As(err, &typ):
		return typ.Field, fmt.Sprintf("wrong type at %s: %v", lineCol(b, typ.Offset), err)
	case errors.Is(err, errTrailing):
		return "", fmt.Sprintf("%v (at %s)", err, lineCol(b, dec.InputOffset()+1))
	}
	if rest, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		name := strings.Trim(rest, `"`)
		return name, fmt.Sprintf("unknown field %q", name)
	}
	return "", err.Error()
}

// lineCol renders the 1-based line:column of the 1-based byte offset off.
func lineCol(b []byte, off int64) string {
	idx := int(off) - 1
	if idx < 0 {
		idx = 0
	}
	if idx > len(b) {
		idx = len(b)
	}
	line := 1 + bytes.Count(b[:idx], []byte{'\n'})
	col := idx - (bytes.LastIndexByte(b[:idx], '\n') + 1) + 1
	return fmt.Sprintf("line %d, column %d", line, col)
}

var _ modding.Source = (*Loader)(nil)
