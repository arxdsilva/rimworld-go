package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Take is one ElevenLabs generation, approved or not (ledger.json).
type Take struct {
	ID       string  `json:"id"`
	Take     int     `json:"take"`
	Part     string  `json:"part"`
	File     string  `json:"file,omitempty"`
	Tool     string  `json:"tool"`
	Seconds  float64 `json:"seconds"`
	Loop     bool    `json:"loop,omitempty"`
	Credits  int     `json:"credits"`
	Approved string  `json:"approved"`
	Date     string  `json:"date"`
	Prompt   string  `json:"prompt,omitempty"`
}

// Ledger is ledger.json. Totals is the person's dashboard reading and is
// carried through untouched.
type Ledger struct {
	Note   string          `json:"note"`
	Takes  []Take          `json:"takes"`
	Totals json.RawMessage `json:"totals,omitempty"`
}

// Take states.
const (
	statePending  = "pending"
	stateRejected = "rejected"
	stateApproved = "approved"
	stateFailed   = "failed" // a refused call, never audio
)

// state classifies Approved: "" and "pending ..." are waiting for a
// listener, "refused ..." is a call that produced nothing.
func (t Take) state() string {
	switch {
	case t.Approved == "" || strings.HasPrefix(t.Approved, "pending"):
		return statePending
	case t.Approved == "rejected":
		return stateRejected
	case strings.HasPrefix(t.Approved, "refused"):
		return stateFailed
	}
	return stateApproved
}

func loadLedger(file string) (*Ledger, error) {
	raw, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return &Ledger{}, nil
	}
	if err != nil {
		return nil, err
	}
	var l Ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// save writes the ledger atomically (a crash mid-run keeps every logged take).
func (l *Ledger) save(file string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(l); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// spent is the credits all logged takes cost.
func (l *Ledger) spent() int {
	n := 0
	for _, t := range l.Takes {
		n += t.Credits
	}
	return n
}

// forID lists id's takes in order.
func (l *Ledger) forID(id string) []int {
	var idx []int
	for i, t := range l.Takes {
		if t.ID == id {
			idx = append(idx, i)
		}
	}
	return idx
}

// nextTake numbers the next take of id.
func (l *Ledger) nextTake(id string) int {
	n := 0
	for _, i := range l.forID(id) {
		n = max(n, l.Takes[i].Take)
	}
	return n + 1
}

// latest is the index of id's newest take with one of the given states (-1: none).
func (l *Ledger) latest(id string, states ...string) int {
	best := -1
	for _, i := range l.forID(id) {
		for _, s := range states {
			if l.Takes[i].state() == s && (best < 0 || l.Takes[i].Take >= l.Takes[best].Take) {
				best = i
			}
		}
	}
	return best
}

// ids lists the distinct animal ids that have a take, in ledger order.
func (l *Ledger) ids() []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range l.Takes {
		if strings.HasPrefix(t.ID, "animal.") && !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, t.ID)
		}
	}
	return out
}
