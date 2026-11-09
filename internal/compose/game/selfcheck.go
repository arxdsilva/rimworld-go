package game

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cligame "rimworld-go/internal/adapters/inbound/cli/game"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/display"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/outbound/ebitenaudio"
	"rimworld-go/internal/platform/assetroot"
)

// langRel is the translation catalogs' folder, as the shell finds it
// (ebiten/shell.go newTexts).
const langRel = "assets/lang"

// configName is the game's folder under os.UserConfigDir (settings, saves,
// logs).
const configName = "rimworld-go"

// Report answers -version and -selfcheck: it prints to out and says the
// exit status; done is false when neither flag was given and the game
// should run. Neither opens a window, starts the log file or creates a
// folder.
func Report(opts cligame.Options, version string, out io.Writer) (code int, done bool) {
	switch {
	case opts.Version:
		fmt.Fprintln(out, version)
		return 0, true
	case opts.SelfCheck:
		return selfCheck(out, version, gameFiles(), configDir), true
	}
	return 0, false
}

// fileCheck is one place the game loads from at start.
type fileCheck struct {
	name string
	find func() (string, error)
}

// gameFiles are the asset roots the client loads, found the way the
// client finds them (assetroot: the working directory, the app bundle's
// Resources, the executable's folder).
func gameFiles() []fileCheck {
	return []fileCheck{
		{"translations", func() (string, error) { return assetroot.Find(filepath.FromSlash(langRel), true) }},
		{"art", art.Locate},
		{"menu splash", mainmenu.LocateMenuSplash},
		{"sounds", ebitenaudio.LocateAudio},
		{"window icon", windowIcons},
	}
}

// windowIcons reads the window icons the way Run does (display.LoadIcons).
func windowIcons() (string, error) {
	icons, err := display.LoadIcons()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d sizes", len(icons)), nil
}

// configDir is the per-user settings folder; it is resolved, not created.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, configName), nil
}

// selfCheck prints each file root and the settings folder, and returns 1
// when one is missing. A settings folder that does not exist yet is fine
// (the first run makes it); one whose place can't be resolved is not.
func selfCheck(out io.Writer, version string, files []fileCheck, config func() (string, error)) int {
	code := 0
	fmt.Fprintf(out, "rimworld-go %s\n", version)
	if exe, err := os.Executable(); err == nil {
		fmt.Fprintf(out, "ok       executable: %s\n", exe)
	}
	for _, f := range files {
		p, err := f.find()
		if err != nil {
			fmt.Fprintf(out, "MISSING  %s: %v\n", f.name, err)
			code = 1
			continue
		}
		fmt.Fprintf(out, "ok       %s: %s\n", f.name, p)
	}
	if dir, err := config(); err != nil {
		fmt.Fprintf(out, "MISSING  settings folder: %v\n", err)
		code = 1
	} else {
		state := "exists"
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			state = "made on first run"
		}
		fmt.Fprintf(out, "ok       settings folder: %s (%s)\n", dir, state)
	}
	if code != 0 {
		fmt.Fprintln(out, "selfcheck failed: reinstall the game, or keep the assets folder next to the executable")
	}
	return code
}
