// Command sfxgen makes the animal voices (SND-003): it sends the prompts in
// assets/source/audio/animal_prompts.json to ElevenLabs text-to-sound-effects,
// logs every take in the ledger, turns the raw takes into game-ready Ogg files
// (ffmpeg + oggenc), writes a review page and, once a person has listened,
// lists the approved clips in the sound list (assets/metadata/audio/audio.json).
//
//	sfxgen -dry-run                      what a run would send and cost (no network, no key)
//	sfxgen [-only ids] [-max-credits N]  generate (paid; key from $ELEVENLABS_API_KEY only)
//	sfxgen -process [-only ids]          raw takes -> sfx/animals/<key>_<event>.ogg (+ _b)
//	sfxgen -review -o page.html          a page with a player for every processed clip
//	sfxgen -approve ids / -reject ids    record the listener's verdict (ledger + sound list)
//
// The key is read from the environment and is never printed, logged or saved.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// keyEnv is the only place the API key may come from.
const keyEnv = "ELEVENLABS_API_KEY"

// config is every flag.
type config struct {
	prompts, ledger, out, assets, baseURL string
	only                                  string
	dryRun, process, review, retry        bool
	budgetFromAPI                         bool
	maxCredits, budget, reserve           int
	creditRate, seconds                   float64
	reviewOut, approve, reject, reviewer  string
	replace                               bool
	backoff                               time.Duration
}

// app is one run: the config plus what tests replace.
type app struct {
	cfg    config
	key    string
	stdout io.Writer
	stderr io.Writer
	client *http.Client
	now    func() time.Time
	// ffmpeg and oggenc are the tool names; lufs measures a file (ffmpeg by default).
	ffmpeg, oggenc string
	lufs           func(path string) (float64, error)
}

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "sfxgen:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	a, err := newApp(args, getenv, stdout, stderr)
	if err != nil || a == nil {
		return err
	}
	return a.dispatch()
}

// newApp parses the flags; a nil app with a nil error means -h was asked.
func newApp(args []string, getenv func(string) string, stdout, stderr io.Writer) (*app, error) {
	fs := flag.NewFlagSet("sfxgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var c config
	fs.StringVar(&c.prompts, "prompts", "assets/source/audio/animal_prompts.json", "prompt list")
	fs.StringVar(&c.only, "only", "", "comma-separated ids or globs (animal.cow.*) to act on")
	fs.BoolVar(&c.dryRun, "dry-run", false, "print what a run would send and cost; no network calls except -budget-from-api")
	fs.StringVar(&c.out, "out", "assets/source/audio/elevenlabs/raw", "where raw takes are saved")
	fs.StringVar(&c.ledger, "ledger", "assets/source/audio/elevenlabs/ledger.json", "take ledger")
	fs.StringVar(&c.assets, "assets", "assets", "assets directory (sound list and generated audio)")
	fs.IntVar(&c.maxCredits, "max-credits", 0, "hard stop: most credits this run may spend (0: no cap)")
	fs.BoolVar(&c.budgetFromAPI, "budget-from-api", false, "read credits from /v1/user/subscription before and after every take")
	fs.IntVar(&c.budget, "budget", 10000, "credit budget (prompts.json budget_credits)")
	fs.IntVar(&c.reserve, "reserve", 1500, "credits never to spend (prompts.json reserve_credits)")
	fs.Float64Var(&c.creditRate, "credit-rate", 10, "estimated credits per second of sound")
	fs.Float64Var(&c.seconds, "seconds", 0, "override every selected sound's length (for the 1 s cost sketch)")
	fs.BoolVar(&c.retry, "retry", false, "generate the one allowed retry of sounds rejected once")
	fs.BoolVar(&c.process, "process", false, "turn approved (or -only) takes into Ogg files")
	fs.BoolVar(&c.review, "review", false, "write the review page (-o)")
	fs.StringVar(&c.reviewOut, "o", "", "-review: HTML output path (default stdout)")
	fs.StringVar(&c.approve, "approve", "", "comma-separated ids or globs a person approved")
	fs.StringVar(&c.reject, "reject", "", "comma-separated ids or globs a person rejected")
	fs.StringVar(&c.reviewer, "reviewer", "arxdsilva", "who listened")
	fs.BoolVar(&c.replace, "replace", false, "-approve: replace a sound-list entry that is not from ElevenLabs (e.g. a placeholder)")
	fs.StringVar(&c.baseURL, "base-url", "https://api.elevenlabs.io", "API base URL (tests point it at httptest)")
	fs.DurationVar(&c.backoff, "backoff", 2*time.Second, "first wait before retrying a 429/5xx (doubles each time)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, nil
		}
		return nil, err
	}
	a := &app{cfg: c, key: getenv(keyEnv), stdout: stdout, stderr: stderr,
		client: &http.Client{Timeout: 2 * time.Minute}, now: time.Now, ffmpeg: "ffmpeg", oggenc: "oggenc"}
	a.lufs = a.measureLUFS
	return a, nil
}

// dispatch runs the mode the flags chose.
func (a *app) dispatch() error {
	c := a.cfg
	modes := 0
	for _, on := range []bool{c.process, c.review, c.approve != "", c.reject != ""} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		return errors.New("pick one of -process, -review, -approve, -reject")
	}
	switch {
	case c.approve != "":
		return a.approveIDs(c.approve)
	case c.reject != "":
		return a.rejectIDs(c.reject)
	case c.review:
		return a.writeReview()
	case c.process:
		return a.processTakes()
	}
	return a.generate()
}
