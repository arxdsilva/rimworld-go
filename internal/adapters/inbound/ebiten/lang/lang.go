// Package lang is the client's translation layer (OPT-001,
// OPTIONS_GAP_ANALYSIS.md "Translation layer"). Keys are the English source
// strings, so English needs no catalog and its output is the source text
// byte for byte. No package state: the shell holds the current catalog in a
// Live and serves it as ports.Texts; pure helpers take a Tr.
//
// GEAR/OPT contract skeleton: English only. The catalog loader, the
// extractor and the untranslated-text guard are W1-LANG's.
package lang

import "fmt"

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
