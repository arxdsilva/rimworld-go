package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
)

type reviewRow struct {
	ID, Prompt, Status string
	Take               int
	Seconds            float64
	Credits            int
	A, B               template.URL // player sources; "" when there is no such file
	LufsA, LufsB       string
}

var reviewTmpl = template.Must(template.New("review").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Animal voices: review</title>
<style>
body{font:14px/1.4 system-ui,sans-serif;margin:1.5rem;max-width:72rem}
table{border-collapse:collapse;width:100%}
th,td{border-bottom:1px solid #ccc;padding:.4rem .5rem;text-align:left;vertical-align:top}
.id{font-family:monospace}.prompt{max-width:22rem}
audio{width:14rem;height:2rem}
</style>
<h1>Animal voices: review</h1>
<p>Listen to every row (A and B), tick approve or reject, then copy the commands below.</p>
<table>
<tr><th>id</th><th>prompt</th><th>A</th><th>B (pitched)</th><th>seconds</th><th>LUFS A / B</th><th>credits</th><th>status</th><th>approve</th><th>reject</th></tr>
{{range .}}<tr data-id="{{.ID}}">
<td class="id">{{.ID}} (take {{.Take}})</td><td class="prompt">{{.Prompt}}</td>
<td>{{if .A}}<audio controls preload="none" src="{{.A}}"></audio>{{end}}</td>
<td>{{if .B}}<audio controls preload="none" src="{{.B}}"></audio>{{end}}</td>
<td>{{printf "%.1f" .Seconds}}</td><td>{{.LufsA}} / {{.LufsB}}</td><td>{{.Credits}}</td><td>{{.Status}}</td>
<td><input type="checkbox" class="ok"></td><td><input type="checkbox" class="no"></td></tr>
{{end}}</table>
<pre id="cmds"></pre>
<script>
function ids(c){return [...document.querySelectorAll('tr[data-id]')].filter(r=>r.querySelector(c).checked).map(r=>r.dataset.id).join(',')}
function show(){const a=ids('.ok'),r=ids('.no');document.getElementById('cmds').textContent=
(a?'go run ./cmd/sfxgen -approve '+a+'\n':'')+(r?'go run ./cmd/sfxgen -reject '+r+'\n':'')}
document.addEventListener('change',show);
</script>
`))

func lufsText(v float64, err error) string {
	if err != nil || math.IsNaN(v) {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", v)
}

// writeReview writes one row per processed clip: the newest take of each
// id that has an Ogg file.
func (a *app) writeReview() error {
	l, err := loadLedger(a.cfg.ledger)
	if err != nil {
		return err
	}
	var rows []reviewRow
	ids := l.ids()
	sort.Strings(ids)
	for _, id := range ids {
		if a.cfg.only != "" && !matchIDs(a.cfg.only, id) {
			continue
		}
		if _, ok := events[eventOf(id)]; !ok {
			continue
		}
		i := l.latest(id, stateApproved, statePending, stateRejected)
		aFile, bFile := a.clipFiles(id)
		if i < 0 {
			continue
		}
		if _, err := os.Stat(aFile); err != nil {
			continue
		}
		t := l.Takes[i]
		row := reviewRow{ID: id, Prompt: t.Prompt, Take: t.Take, Seconds: t.Seconds, Credits: t.Credits, Status: t.state(),
			A: a.playerSrc(aFile), LufsA: lufsText(a.lufs(aFile))}
		if t.state() == stateApproved {
			row.Status = "approved by " + t.Approved
		}
		if _, err := os.Stat(bFile); err == nil {
			row.B, row.LufsB = a.playerSrc(bFile), lufsText(a.lufs(bFile))
		} else {
			row.LufsB = "-"
		}
		rows = append(rows, row)
	}
	var buf bytes.Buffer
	if err := reviewTmpl.Execute(&buf, rows); err != nil {
		return err
	}
	if a.cfg.reviewOut == "" {
		_, err := a.stdout.Write(buf.Bytes())
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.cfg.reviewOut), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(a.cfg.reviewOut, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "review page: %s (%d clips)\n", a.cfg.reviewOut, len(rows))
	return nil
}

// playerSrc is the audio file as the page refers to it: relative to the
// page, or a file:// URL when the page goes to stdout.
func (a *app) playerSrc(file string) template.URL {
	abs, err := filepath.Abs(file)
	if err != nil {
		return template.URL((&url.URL{Path: filepath.ToSlash(file)}).String())
	}
	if a.cfg.reviewOut != "" {
		if page, err := filepath.Abs(a.cfg.reviewOut); err == nil {
			if rel, err := filepath.Rel(filepath.Dir(page), abs); err == nil {
				return template.URL((&url.URL{Path: filepath.ToSlash(rel)}).String())
			}
		}
	}
	return template.URL((&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String())
}
