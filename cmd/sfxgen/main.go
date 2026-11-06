// Command sfxgen makes the animal voices (SND-003): it turns the prompts in
// assets/source/audio/animal_prompts.json into takes, logs every take in the
// ledger, turns the raw takes into game-ready Ogg files (ffmpeg + oggenc),
// writes a review page and, once a person has listened, lists the approved
// clips in the sound list (assets/metadata/audio/audio.json).
//
// Two backends make the takes (-backend):
//
//   - moss (the default, user decision 2026-10-31): the open-source
//     MOSS-SoundEffect v2.0 model (Apache-2.0) run locally through
//     cmd/sfxgen/moss/generate.py. Free; each take is processed to Ogg at
//     once. A take needs about 4 min and 16-22 GB of memory on an Apple
//     M-series GPU, so takes run one at a time and wait until
//     -moss-min-free-gb is free.
//   - elevenlabs: the paid text-to-sound-effects API, only with
//     user-approved spending (dry-run, approval, ledger, reserve).
//
// MOSS setup, outside the repo (defaults under $XDG_CACHE_HOME/rimworld-go/moss,
// else ~/.cache/rimworld-go/moss; override with -moss-python/-moss-model or
// $SFXGEN_MOSS_PYTHON/$SFXGEN_MOSS_MODEL):
//
//	python3.12 -m venv ~/.cache/rimworld-go/moss/venv
//	~/.cache/rimworld-go/moss/venv/bin/pip install <MOSS-TTS repo checkout> soundfile
//	huggingface-cli download OpenMOSS-Team/MOSS-SoundEffect-v2.0 \
//	    --local-dir ~/.cache/rimworld-go/moss/MOSS-SoundEffect-v2.0
//
// Usage:
//
//	sfxgen -dry-run [-only ids]            the plan (moss: time and command; elevenlabs: credits)
//	sfxgen [-only ids]                     generate with MOSS and process (free, local)
//	sfxgen -supersede elevenlabs [-only ids]
//	                                       also make sounds again whose approved take came from that backend
//	sfxgen -backend elevenlabs [-only ids] [-max-credits N]
//	                                       paid; key from $ELEVENLABS_API_KEY only
//	sfxgen -process [-only ids]            raw takes -> sfx/animals/<key>_<event>.ogg (+ _b)
//	sfxgen -review -o page.html            a page with a player for every processed clip
//	sfxgen -approve ids / -reject ids      record the listener's verdict (ledger + sound list)
//
// Every take records its backend in the ledger. The ElevenLabs key is read
// from the environment and is never printed, logged or saved.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// defaultOut is where raw takes go by backend (both gitignored).
var defaultOut = map[string]string{
	backendMoss:       "assets/source/audio/moss/raw",
	backendElevenLabs: "assets/source/audio/elevenlabs/raw",
}

// keyEnv is the only place the API key may come from.
const keyEnv = "ELEVENLABS_API_KEY"

// config is every flag.
type config struct {
	prompts, ledger, out, assets, baseURL string
	only, supersede                       string
	dryRun, process, review, retry        bool
	budgetFromAPI                         bool
	maxCredits, budget, reserve           int
	creditRate, seconds                   float64
	reviewOut, approve, reject, reviewer  string
	license                               string // -approve: the ElevenLabs plan the takes were made under
	replace                               bool
	backoff                               time.Duration
	backend                               string
	moss                                  mossConfig
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
	// runMoss runs one MOSS command line; freeGB reads free memory; sleep
	// waits between memory checks.
	runMoss func(ctx context.Context, argv []string) ([]byte, error)
	freeGB  func() (float64, bool)
	sleep   func(time.Duration)
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
	fs.StringVar(&c.backend, "backend", backendMoss, "who makes the takes: moss (local, free) or elevenlabs (paid, user-approved spending)")
	fs.StringVar(&c.out, "out", "", "where raw takes are saved (default assets/source/audio/<backend>/raw)")
	fs.StringVar(&c.ledger, "ledger", "assets/source/audio/elevenlabs/ledger.json", "take ledger")
	fs.StringVar(&c.assets, "assets", "assets", "assets directory (sound list and generated audio)")
	fs.IntVar(&c.maxCredits, "max-credits", 0, "hard stop: most credits this run may spend (0: no cap)")
	fs.BoolVar(&c.budgetFromAPI, "budget-from-api", false, "read credits from /v1/user/subscription before and after every take")
	fs.IntVar(&c.budget, "budget", 10000, "credit budget (prompts.json budget_credits)")
	fs.IntVar(&c.reserve, "reserve", 1500, "credits never to spend (prompts.json reserve_credits)")
	fs.Float64Var(&c.creditRate, "credit-rate", 10, "estimated credits per second of sound")
	fs.Float64Var(&c.seconds, "seconds", 0, "override every selected sound's length (for the 1 s cost sketch)")
	fs.BoolVar(&c.retry, "retry", false, "generate the one allowed retry of sounds rejected once")
	fs.StringVar(&c.supersede, "supersede", "", "make sounds again even if approved, when the approved take came from this backend")
	fs.BoolVar(&c.process, "process", false, "turn approved (or -only) takes into Ogg files")
	fs.BoolVar(&c.review, "review", false, "write the review page (-o)")
	fs.StringVar(&c.reviewOut, "o", "", "-review: HTML output path (default stdout)")
	fs.StringVar(&c.approve, "approve", "", "comma-separated ids or globs a person approved")
	fs.StringVar(&c.reject, "reject", "", "comma-separated ids or globs a person rejected")
	fs.StringVar(&c.reviewer, "reviewer", "arxdsilva", "who listened")
	fs.StringVar(&c.license, "license", "", "-approve of ElevenLabs takes: the paid plan's terms (free-tier output is non-commercial and cannot ship)")
	fs.BoolVar(&c.replace, "replace", false, "-approve: replace a sound-list entry that is not from ElevenLabs (e.g. a placeholder)")
	fs.StringVar(&c.baseURL, "base-url", "https://api.elevenlabs.io", "API base URL (tests point it at httptest)")
	fs.DurationVar(&c.backoff, "backoff", 2*time.Second, "first wait before retrying a 429/5xx (doubles each time)")
	python, model := mossDefaults(getenv)
	m := &c.moss
	fs.StringVar(&m.python, "moss-python", python, "Python of the MOSS virtualenv (env "+mossPythonEnv+")")
	fs.StringVar(&m.model, "moss-model", model, "MOSS-SoundEffect v2.0 snapshot directory (env "+mossModelEnv+")")
	fs.StringVar(&m.runner, "moss-runner", "cmd/sfxgen/moss/generate.py", "the MOSS runner script")
	fs.StringVar(&m.device, "moss-device", "auto", "torch device: auto, mps, cuda or cpu")
	fs.IntVar(&m.steps, "moss-steps", 100, "diffusion steps per take")
	fs.Float64Var(&m.cfgScale, "moss-cfg", 4.0, "classifier-free guidance scale")
	fs.Int64Var(&m.seed, "moss-seed", 0, "seed for every take (0: a stable hash of id and take number)")
	fs.Float64Var(&m.minFreeGB, "moss-min-free-gb", 24, "wait until this much memory is free + inactive before a take (0: never wait)")
	fs.DurationVar(&m.poll, "moss-poll", 30*time.Second, "how often to recheck free memory")
	fs.DurationVar(&m.maxWait, "moss-max-wait", 30*time.Minute, "give up when memory stays short this long")
	fs.DurationVar(&m.timeout, "moss-timeout", 20*time.Minute, "longest one take may run")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, nil
		}
		return nil, err
	}
	if _, ok := defaultOut[c.backend]; !ok {
		return nil, fmt.Errorf("-backend %q: want %s or %s", c.backend, backendMoss, backendElevenLabs)
	}
	if c.out == "" {
		c.out = defaultOut[c.backend]
	}
	a := &app{cfg: c, key: getenv(keyEnv), stdout: stdout, stderr: stderr,
		client: &http.Client{Timeout: 2 * time.Minute}, now: time.Now, ffmpeg: "ffmpeg", oggenc: "oggenc",
		runMoss: execMoss, freeGB: freeMemoryGB, sleep: time.Sleep}
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
