package savefmt

import "slices"

// Compat is how this build reads a save (ports.SaveSlot.Compat; 0 fine).
type Compat int

const (
	CompatOK      Compat = iota // same format, or another build's content
	CompatOlder                 // an older format: Load migrates it
	CompatNewer                 // written by a newer build: Load refuses it
	CompatContent               // same format and build, other content: loads with a notice
	CompatTooOld                // older than any format Load still reads: refused
)

// Loads reports whether Load reads a save of this compat.
func (c Compat) Loads() bool { return c != CompatNewer && c != CompatTooOld }

// Runtime is what this build is, for CompatOf: the save format it writes,
// the oldest it reads, its build and its content.
type Runtime struct {
	Version, Min int
	Build        string
	Content      Content
}

// CompatOf is how a build at rt reads a save with header info (SAV-021).
// A content difference only counts between saves of the same format and
// build: another build's content differs anyway, and its format decides.
func CompatOf(info Info, rt Runtime) Compat {
	switch {
	case info.Version > rt.Version:
		return CompatNewer
	case info.Version < rt.Min:
		return CompatTooOld
	case info.Version < rt.Version:
		return CompatOlder
	case info.Build == rt.Build && info.Content.Hash != rt.Content.Hash:
		return CompatContent
	}
	return CompatOK
}

// MissingPlugins lists the plugins info was played with that rt does not
// run, in the save's load order (SAV-025).
func MissingPlugins(info Info, rt Runtime) []string {
	var out []string
	for _, p := range info.Content.Plugins {
		if !slices.Contains(rt.Content.Plugins, p) {
			out = append(out, p)
		}
	}
	return out
}
