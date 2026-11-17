package modloader

import "rimworld-go/internal/application/modding"

// Loader reads the plugins under its roots. It implements modding.Source.
type Loader struct {
	roots []string
}

// New makes a Loader over roots (missing roots are skipped). Owner: W1-load.
func New(roots ...string) *Loader { return &Loader{roots: roots} }

// Plugins lists every plugin folder under the roots, sorted by id. A
// folder whose manifest cannot be read is still listed, with its
// problems, under its folder name. Owner: W1-load.
func (l *Loader) Plugins() ([]modding.Plugin, error) { return nil, nil }

var _ modding.Source = (*Loader)(nil)
