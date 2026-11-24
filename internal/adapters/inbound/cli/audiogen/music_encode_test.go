// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package audiogen

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"rimworld-go/pkg/audiomanifest"
)

// TestComposeAllEncodeFailureAndTempWavMode runs composeAll against a fake
// ffmpeg that checks the temp wav is mode 0o600 and then fails: the wav write,
// the output MkdirAll and the encode error path all run, and the manifest is
// left untouched.
func TestComposeAllEncodeFailureAndTempWavMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	assets := t.TempDir()
	if err := writeManifest(assets, audiomanifest.Manifest{}); err != nil {
		t.Fatal(err)
	}
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "mode-ok")
	script := "#!/bin/sh\nPATH=/usr/bin:/bin\n" +
		"while [ $# -gt 0 ]; do [ \"$1\" = -i ] && wav=$2; shift; done\n" +
		"[ -n \"$(find \"$wav\" -perm 0600)\" ] && : > '" + marker + "'\n" +
		"echo boom >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte(script), 0o700); err != nil { // #nosec G306 -- test fake tool must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	err := composeAll(assets)
	if err == nil || !strings.Contains(err.Error(), "encode ") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("composeAll error = %v, want an encode failure carrying ffmpeg's output", err)
	}
	if _, serr := os.Stat(marker); serr != nil {
		t.Fatalf("temp wav was not mode 0o600: %v", serr)
	}
	if _, serr := os.Stat(filepath.Join(assets, filepath.FromSlash(audiomanifest.Dir), "music")); serr != nil {
		t.Fatalf("output dir not created: %v", serr)
	}
}

func TestComposeAllNeedsFFmpeg(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := composeAll(t.TempDir()); err == nil || !strings.Contains(err.Error(), "needs ffmpeg") {
		t.Fatalf("composeAll error = %v, want the missing-ffmpeg error", err)
	}
}
