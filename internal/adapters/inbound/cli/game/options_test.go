package game

import (
	"bytes"
	"errors"
	"flag"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/application/devscene"
)

func TestOnlyGivenGraphicsFlagsOverride(t *testing.T) {
	t.Parallel()
	o, err := Parse([]string{"game"})
	if err != nil || o.HiDPI != nil || o.VSync != nil || o.SeedSet || o.Seed != 1 || o.Hour != -1 || o.SaveDir != "" {
		t.Fatalf("defaults: %+v %v", o, err)
	}
	o, err = Parse([]string{"game", "-hidpi=false", "-vsync", "-seed", "7", "-scenario", "zoo", "-save-dir", "bin/saves"})
	if err != nil || o.SaveDir != "bin/saves" || o.HiDPI == nil || *o.HiDPI || o.VSync == nil || !*o.VSync || !o.SeedSet || o.Seed != 7 || o.Scenario != "zoo" {
		t.Fatalf("given: %+v %v", o, err)
	}
}

func TestBadFlagsAndHelpExitAsTheFlagPackageDoes(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	_, err := parse([]string{"bin/game", "-h"}, &out)
	if !errors.Is(err, flag.ErrHelp) || ExitCode(err) != 0 || !strings.HasPrefix(out.String(), "Usage of bin/game:\n") {
		t.Errorf("-h: %v, %q", err, out.String()[:min(40, out.Len())])
	}
	out.Reset()
	if _, err := parse([]string{"bin/game", "-nope"}, &out); err == nil || ExitCode(err) != 2 || !strings.Contains(out.String(), "-nope") {
		t.Errorf("-nope: %v, %q", err, out.String())
	}
}

// Every scenario the -scenario help names is one devscene builds.
func TestHelpedScenariosExist(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	parse([]string{"game", "-h"}, &out)
	help := out.String()
	at := strings.Index(help, "-scenario string")
	if at < 0 {
		t.Fatal("no -scenario in the help")
	}
	line := strings.SplitN(help[at:], "\n", 3)[1]
	names := regexp.MustCompile(`(?:^|\| )([a-z]+) \(`).FindAllStringSubmatch(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "dev: start scenario:")), -1)
	if len(names) < 20 {
		t.Fatalf("read %d scenario names from %q", len(names), line)
	}
	for _, m := range names {
		if !devscene.Known(m[1]) {
			t.Errorf("-scenario %s is in the help but devscene does not build it", m[1])
		}
	}
}
