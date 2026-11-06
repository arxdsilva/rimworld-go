package arch

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The agent-guidance guard (AI-001). All agent guidance lives in .agents/
// (.agents/README.md); the tool folders keep only what each tool cannot
// find elsewhere. These tests keep paths, links, skill links, ledgers and the
// size of the always-loaded root AGENTS.md from drifting.

// Text files the guard reads; everything else (images, sounds, binaries) is skipped.
var guardExts = map[string]bool{
	".go": true, ".md": true, ".py": true, ".sh": true, ".yml": true,
	".yaml": true, ".txt": true, ".json": true, ".mdc": true, ".toml": true,
}

// Directories never scanned: VCS data, build output and vendored code.
var guardSkipDirs = map[string]bool{
	".git": true, "bin": true, "dist": true, "node_modules": true,
	"third_party": true, ".venv": true,
}

type guardFile struct {
	rel  string // slash-separated, relative to the module root
	body string
}

// guardFiles reads every text file of the repository once.
func guardFiles(t *testing.T, root string) []guardFile {
	t.Helper()
	var out []guardFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (guardSkipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".tmp")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if !guardExts[filepath.Ext(path)] && d.Name() != "Makefile" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, guardFile{filepath.ToSlash(rel), string(b)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Outside .agents/ (whose ledgers record history), nothing names the old
// tool-specific homes; a line may name Cursor's folder only to say it
// holds mcp.json.
func TestNoStaleAgentPaths(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	cursor := ".cur" + "sor/"     // split so this file does not match itself
	hooks := ".claude/" + "hooks" // likewise
	for _, f := range guardFiles(t, root) {
		if strings.HasPrefix(f.rel, ".agents/") {
			continue
		}
		for n, line := range strings.Split(f.body, "\n") {
			if strings.Contains(line, cursor) && !strings.Contains(line, "mcp.json") {
				t.Errorf("%s:%d names %s; agent guidance lives in .agents/ (.agents/README.md)", f.rel, n+1, cursor)
			}
			if strings.Contains(line, hooks) {
				t.Errorf("%s:%d names %s; the hooks are in .agents/hooks/", f.rel, n+1, hooks)
			}
		}
	}
}

var (
	mdLink      = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	tickedPath  = regexp.MustCompile("`(\\.agents/[^`\\s]*)`")
	ledgerCite  = regexp.MustCompile(`\.agents/handoffs/([A-Z]+\.md)#([a-z0-9_.-]*[a-z0-9_])`)
	ledgerTitle = regexp.MustCompile(`· \d{4}-\d{2}-\d{2} ·`)
)

// Every relative link in agent guidance resolves, every backticked .agents/
// path exists, and every ledger anchor cited anywhere names a real section.
func TestAgentLinksResolve(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	heads := ledgerHeadings(t, root)
	for _, f := range guardFiles(t, root) {
		guidance := strings.HasPrefix(f.rel, ".agents/") && strings.HasSuffix(f.rel, ".md") ||
			filepath.Base(f.rel) == "AGENTS.md"
		for _, m := range ledgerCite.FindAllStringSubmatch(f.body, -1) {
			if !heads[m[1]][m[2]] {
				t.Errorf("%s cites %s#%s, which is not a section heading", f.rel, m[1], m[2])
			}
		}
		if !guidance {
			continue
		}
		dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(f.rel)))
		for _, m := range mdLink.FindAllStringSubmatch(f.body, -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			path, anchor, _ := strings.Cut(target, "#")
			abs := filepath.Join(dir, filepath.FromSlash(path))
			if _, err := os.Stat(abs); err != nil {
				t.Errorf("%s links %s, which does not exist", f.rel, target)
				continue
			}
			if rel, _ := filepath.Rel(root, abs); anchor != "" && strings.HasPrefix(filepath.ToSlash(rel), ".agents/handoffs/") {
				if !heads[filepath.Base(abs)][anchor] {
					t.Errorf("%s links %s, which is not a section heading", f.rel, target)
				}
			}
		}
		for _, m := range tickedPath.FindAllStringSubmatch(f.body, -1) {
			path, _, _ := strings.Cut(m[1], "#")
			path = strings.TrimRight(path, ".,;:")
			if strings.ContainsAny(path, "*<>{}…") {
				continue // a pattern, not a path
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				t.Errorf("%s names `%s`, which does not exist", f.rel, m[1])
			}
		}
	}
}

// ledgerHeadings maps each ledger file name to its lowercased section headings.
func ledgerHeadings(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	dir := filepath.Join(root, ".agents", "handoffs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "INDEX.md" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		set := map[string]bool{}
		for line := range strings.Lines(string(b)) {
			if h, ok := strings.CutPrefix(line, "## "); ok {
				set[strings.ToLower(strings.TrimSpace(h))] = true
			}
		}
		out[e.Name()] = set
	}
	return out
}

// Claude Code finds project skills only in .claude/skills/, so each skill in
// .agents/skills/ has exactly one link there, and nothing else lives there.
func TestSkillLinksMatch(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	want := dirNames(t, filepath.Join(root, ".agents", "skills"))
	got := dirNames(t, filepath.Join(root, ".claude", "skills"))
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Fatalf(".claude/skills %v must match .agents/skills %v (one relative symlink each)", got, want)
	}
	for _, name := range want {
		link := filepath.Join(root, ".claude", "skills", name)
		dest, err := os.Readlink(link)
		if err != nil {
			t.Errorf(".claude/skills/%s must be a symlink: %v", name, err)
			continue
		}
		if filepath.ToSlash(dest) != "../../.agents/skills/"+name {
			t.Errorf(".claude/skills/%s points to %s, want ../../.agents/skills/%s", name, dest, name)
		}
		if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
			t.Errorf(".claude/skills/%s does not resolve to a SKILL.md: %v", name, err)
		}
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// Root AGENTS.md loads in every session of every tool, so its size has a
// budget that only shrinks: growth fails, and a budget more than 5% above the
// file asks to be lowered.
func TestRootAgentsBudget(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	budget := readAgentsBudget(t, filepath.Join(root, "internal", "arch", "agents_budget.txt"))
	size := len(b)
	switch {
	case size > budget:
		t.Errorf("root AGENTS.md is %d bytes, over its %d-byte budget: move detail to its canonical home in .agents/ and leave a pointer", size, budget)
	case size*100 < budget*95:
		t.Errorf("root AGENTS.md is %d bytes; the %d-byte budget is stale, lower internal/arch/agents_budget.txt to %d", size, budget, size)
	}
}

func readAgentsBudget(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("%s: %q is not a byte count", path, line)
		}
		return n
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("%s holds no byte count", path)
	return 0
}

// Every ledger section has its title line, appears once across all ledgers
// and has a row in INDEX.md; every INDEX row names a real section.
func TestLedgersAndIndex(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	dir := filepath.Join(root, ".agents", "handoffs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	where := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if name == "AGENTS.md" {
			t.Errorf(".agents/handoffs/AGENTS.md would load as instructions; rename the ledger")
		}
		if name == "INDEX.md" {
			continue
		}
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			t.Errorf(".agents/handoffs/%s: only ledger files (<FAMILY>.md) and INDEX.md belong here", name)
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			h, ok := strings.CutPrefix(line, "## ")
			if !ok {
				continue
			}
			h = strings.TrimSpace(h)
			if prev, dup := where[h]; dup {
				t.Errorf("section %s is in both %s and %s", h, prev, name)
			}
			where[h] = name
			if i+1 >= len(lines) || !ledgerTitle.MatchString(lines[i+1]) {
				t.Errorf("%s: section %s needs the line `<Title> · <YYYY-MM-DD> · <commit>` under its heading (.agents/templates/HANDOFF.md)", name, h)
			}
		}
	}
	index, err := os.ReadFile(filepath.Join(dir, "INDEX.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]bool{}
	for line := range strings.Lines(string(index)) {
		cells := strings.Split(line, "|")
		if len(cells) < 6 || !strings.Contains(cells[4], "#") {
			continue
		}
		id := strings.TrimSpace(cells[1])
		rows[id] = true
		if where[id] == "" {
			t.Errorf("INDEX.md lists %s, which no ledger has (python3 scripts/handoff_index.py)", id)
		}
	}
	for h, ledger := range where {
		if !rows[h] {
			t.Errorf("%s#%s has no INDEX.md row (python3 scripts/handoff_index.py)", ledger, strings.ToLower(h))
		}
	}
}
