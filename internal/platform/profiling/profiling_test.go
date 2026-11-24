// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package profiling

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The profiles land where the flags say, and empty paths write nothing.
func TestProfilesAreWrittenWhereAsked(t *testing.T) {
	dir := t.TempDir()
	trace, heap := filepath.Join(dir, "run.trace"), filepath.Join(dir, "heap.prof")
	Start("", trace)()
	WriteHeap(heap)
	WriteHeap("")
	for _, p := range []string{trace, heap} {
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			t.Errorf("%s: %v", p, err)
		}
	}
}

// A Close failure is logged, not dropped. Changes the process-wide logger, so
// it is not parallel.
func TestCloseLoggedReportsAFailedClose(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "p.prof"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	closeLogged("trace", f)
	if got := buf.String(); !strings.Contains(got, "trace: close:") {
		t.Errorf("log = %q, want a trace close failure", got)
	}
}
