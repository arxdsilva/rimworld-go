package dossier

import "fmt"

// setupTabLabels are the setup modal's tab names in English, for the tests'
// subtest names; the modal draws them through the catalog.
var setupTabLabels = []string{"Look", "Sections", "Presets", "Colours"}

// The English reasons the Look tab gives; the tab says them through the catalog.
const (
	whyNoRestyle = "Needs \"Restyle the pane\": a plain pane cannot hold the card."
	whyDocked    = "The card is docked in the pane, so it has no place of its own."
	whyNoCompact = "Needs \"Compress notifications\" and a floating card."
)

// T and Tf make the fake shell an English catalog.
func (s *fakeShell) T(src string) string { return src }

func (s *fakeShell) Tf(src string, args ...any) string { return fmt.Sprintf(src, args...) }
