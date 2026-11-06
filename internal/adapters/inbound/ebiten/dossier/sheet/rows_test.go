package sheet

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
)

type gapRow struct {
	n      int
	status string
	work   string
	text   string
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for ; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			t.Fatal("no go.mod above the package")
		}
	}
}

func readGapDoc(t *testing.T) (rows []gapRow, doc string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".cursor", "RIMHUD_GAP_ANALYSIS.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc = string(b)
	idRe := regexp.MustCompile(`^RHUD-(\d{3})$`)
	for _, ln := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(ln, "| RHUD-") {
			continue
		}
		cols := strings.Split(ln, "|")
		if len(cols) != 7 {
			t.Errorf("%q: %d columns, want 5", ln[:min(len(ln), 40)], len(cols)-2)
			continue
		}
		m := idRe.FindStringSubmatch(strings.TrimSpace(cols[1]))
		if m == nil {
			t.Errorf("bad id %q", cols[1])
			continue
		}
		n, _ := strconv.Atoi(m[1])
		rows = append(rows, gapRow{n: n, status: strings.TrimSpace(cols[4]), work: cols[5], text: ln})
	}
	return rows, doc
}

// TestEveryRHUDRowIsClassified: .cursor/RIMHUD_GAP_ANALYSIS.md lists every
// feature of the mod this card is measured against. Ids run in order with no
// repeat, each status is one of four, every row has five columns, a row that
// is neither done nor excluded names the package that finishes it, the counts
// table agrees with the rows, DOS-2's rows are all open rows that mention it,
// and the contract's widget key list is what the code has.
func TestEveryRHUDRowIsClassified(t *testing.T) {
	t.Parallel()
	rows, doc := readGapDoc(t)
	if len(rows) == 0 {
		t.Fatal("no rows found")
	}
	pkg := regexp.MustCompile(`DOS-\d`)
	counts := map[string]int{}
	unclassified := 0
	byID := map[int]gapRow{}
	for i, r := range rows {
		if r.n != i+1 {
			t.Fatalf("row %d is out of order or doubled (position %d)", r.n, i+1)
		}
		byID[r.n] = r
		switch r.status {
		case "done", "partial", "missing", "excluded":
			counts[r.status]++
		default:
			t.Errorf("RHUD-%03d: status %q", r.n, r.status)
		}
		if (r.status == "partial" || r.status == "missing") && !pkg.MatchString(r.work) {
			t.Errorf("RHUD-%03d is %s and names no DOS package", r.n, r.status)
		}
		if strings.Contains(r.text, "UNCLASSIFIED") {
			unclassified++
		}
	}
	table := map[string]int{"Rows": len(rows), "Done": counts["done"], "Partial": counts["partial"],
		"Missing": counts["missing"], "Excluded": counts["excluded"], "Marked UNCLASSIFIED": unclassified}
	for label, want := range table {
		re := regexp.MustCompile(`(?m)^\| ` + regexp.QuoteMeta(label) + `[^|]*\| (\d+) \|`)
		m := re.FindStringSubmatch(doc)
		if m == nil {
			t.Errorf("counts table has no %q row", label)
			continue
		}
		if got, _ := strconv.Atoi(m[1]); got != want {
			t.Errorf("counts table says %s = %d, the rows say %d", label, got, want)
		}
	}

	// DOS-2's own rows
	_, perPkg, found := strings.Cut(doc, "### Rows per package")
	line := regexp.MustCompile(`(?m)^\| DOS-2 \| ([0-9, –]+) \|$`).FindStringSubmatch(perPkg)
	if !found || line == nil {
		t.Fatal("no DOS-2 line in Rows per package")
	}
	ids, err := parseIDs(line[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 100 {
		t.Fatalf("DOS-2 has %d rows, expected over a hundred", len(ids))
	}
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			t.Errorf("DOS-2 names RHUD-%03d, which does not exist", id)
			continue
		}
		if r.status == "excluded" {
			t.Errorf("RHUD-%03d is excluded and yet in DOS-2", id)
		}
		if !strings.Contains(r.work, "DOS-2") && !strings.Contains(r.work, "sheet.") {
			t.Errorf("RHUD-%03d is in DOS-2's list and its work column does not mention DOS-2 or sheet", id)
		}
	}

	// the contract's widget key list
	m := regexp.MustCompile("Widget keys: `([a-z_ ]+)`").FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("no widget key list in the contract")
	}
	var keys []string
	keys = append(keys, strings.Fields(m[1])...)
	if !reflect.DeepEqual(keys, WidgetKeys()) {
		t.Errorf("the contract lists %v, WidgetKeys() has %v", keys, WidgetKeys())
	}
	for _, k := range keys {
		if _, ok := Lookup(k); !ok {
			t.Errorf("widget key %s is not in the catalog", k)
		}
	}
}

// parseIDs reads "009, 061–073, 075" into numbers.
func parseIDs(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		lo, hi, isRange := strings.Cut(part, "–")
		a, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			return nil, fmt.Errorf("bad id %q", part)
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(strings.TrimSpace(hi)); err != nil || b < a {
				return nil, fmt.Errorf("bad range %q", part)
			}
		}
		for i := a; i <= b; i++ {
			out = append(out, i)
		}
	}
	return out, nil
}

func TestParseIDs(t *testing.T) {
	t.Parallel()
	got, err := parseIDs("009, 061–063, 075")
	if err != nil || !reflect.DeepEqual(got, []int{9, 61, 62, 63, 75}) {
		t.Fatalf("%v %v", got, err)
	}
	for _, bad := range []string{"x", "5–3", "1–"} {
		if _, err := parseIDs(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

// RHUD-284: a trainable's tooltip is the animal's summary, then what it teaches.
func TestTrainTipStartsWithTheAnimal(t *testing.T) {
	t.Parallel()
	a := game.AnimalView{Name: "Rex", Label: "dustbuck", Tame: true}
	tip := TrainTip(en, a, game.AnimalTrainingView{Label: "Obedience", Needed: 3})
	if len(tip) < 3 || !strings.HasPrefix(tip[0].Text, "Rex, dustbuck") {
		t.Fatalf("tip %+v", tip)
	}
	var taught bool
	for _, l := range tip {
		taught = taught || l.Text == "Obedience"
	}
	if !taught {
		t.Fatalf("no line for the skill: %+v", tip)
	}
}
