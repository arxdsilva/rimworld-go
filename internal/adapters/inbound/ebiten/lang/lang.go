// Package lang is the client's translation layer (OPT-001,
// OPTIONS_GAP_ANALYSIS.md "Translation layer"). Keys are the English source
// strings, so English needs no catalog and its output is the source text
// byte for byte. No package state: the shell holds the current catalog in a
// Live and serves it as ports.Texts; pure helpers take a Tr.
//
// Catalogs are assets/lang/<code>.json (catalog.go). The tests keep them
// honest: TestCatalogIsCurrent regenerates en.json from the T/Tf calls,
// TestUITextIsTranslatable counts text drawn without T/Tf, and
// TestCatalogGlyphs checks the UI font can draw every catalog.
package lang

import (
	"fmt"

	"rimworld-go/pkg/msgfmt"
)

// Tr translates a source string; an unknown key returns src.
type Tr interface {
	T(src string) string
}

// English is the identity catalog.
type English struct{}

// T returns src.
func (English) T(src string) string { return src }

// Tf formats src's translation with args (fmt verbs, kept by translators).
func Tf(tr Tr, src string, args ...any) string { return fmt.Sprintf(tr.T(src), args...) }

// Msg is a simulation message or letter in tr's language: text when m
// has no format (an older save, or a post without one), else m's format
// translated and joined with its arguments, each passed through tr too (a
// kind name or a reason that is a key is translated; a number or a name
// that is not stays). In English it is text byte for byte.
func Msg(tr Tr, m msgfmt.Text, text string) string {
	if m.Format == "" {
		return text
	}
	args := m.Args()
	for i, a := range args {
		args[i] = tr.T(a)
	}
	return msgfmt.Join(tr.T(m.Format), args)
}

// Live is the client's current catalog; Set swaps it when the Language
// option changes, and every holder of the Live sees the new one. The zero
// Live and a nil one translate as English.
type Live struct{ tr Tr }

// Set makes tr the current catalog (nil: English).
func (l *Live) Set(tr Tr) { l.tr = tr }

// T translates src with the current catalog.
func (l *Live) T(src string) string {
	if l == nil || l.tr == nil {
		return src
	}
	return l.tr.T(src)
}

// Tf formats src's translation with args.
func (l *Live) Tf(src string, args ...any) string { return fmt.Sprintf(l.T(src), args...) }
