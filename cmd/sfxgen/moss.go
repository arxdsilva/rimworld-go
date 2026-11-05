package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Backends: where a take comes from. MOSS is the default since the user's
// decision of 2026-10-31; ElevenLabs stays for user-approved paid runs.
const (
	backendMoss       = "moss"
	backendElevenLabs = "elevenlabs"
)

// MOSS provenance for the ledger and the sound list.
const (
	mossTool    = "moss_soundeffect_v2"
	mossLicense = "MOSS-SoundEffect v2.0 model (Apache-2.0), generated locally"
	// mossClipSeconds is about how long one take of 100 steps takes on an
	// Apple M-series GPU; time scales with steps. The model always denoises a
	// 30 s latent, whatever the length: a shorter max_inference_seconds turns
	// the output to noise, so it is not a way to go faster.
	mossClipSeconds = 240
)

// Environment variables that override the default MOSS paths.
const (
	mossPythonEnv = "SFXGEN_MOSS_PYTHON"
	mossModelEnv  = "SFXGEN_MOSS_MODEL"
)

// mossConfig is the -moss-* flags.
type mossConfig struct {
	python, model, runner, device string
	steps                         int
	cfgScale, minFreeGB           float64
	seed                          int64
	poll, maxWait, timeout        time.Duration
}

// mossHome is where the virtualenv and the model live by default:
// $XDG_CACHE_HOME/rimworld-go/moss, else ~/.cache/rimworld-go/moss. Never
// inside the repo.
func mossHome(getenv func(string) string) string {
	base := getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "rimworld-go", "moss")
}

// mossDefaults are the default python and model paths: the environment
// first, then mossHome.
func mossDefaults(getenv func(string) string) (python, model string) {
	home := mossHome(getenv)
	python, model = getenv(mossPythonEnv), getenv(mossModelEnv)
	if python == "" {
		python = filepath.Join(home, "venv", "bin", "python")
	}
	if model == "" {
		model = filepath.Join(home, "MOSS-SoundEffect-v2.0")
	}
	return python, model
}

// mossSeed is the seed of take n of id: -moss-seed when set, else a stable
// hash, so a take can be made again.
func (a *app) mossSeed(id string, n int) int64 {
	if a.cfg.moss.seed != 0 {
		return a.cfg.moss.seed
	}
	h := fnv.New32a()
	h.Write([]byte(id + "/take" + strconv.Itoa(n)))
	return int64(h.Sum32() & 0x7fffffff)
}

// mossCommand is the command line that makes one take: the venv's Python,
// isolated (-I), running the runner script.
func (a *app) mossCommand(text string, seconds float64, seed int64, wav string) []string {
	m := a.cfg.moss
	return []string{m.python, "-I", m.runner,
		"--model", m.model,
		"--prompt", text,
		"--seconds", strconv.FormatFloat(seconds, 'f', 1, 64),
		"--seed", strconv.FormatInt(seed, 10),
		"--steps", strconv.Itoa(m.steps),
		"--cfg", strconv.FormatFloat(m.cfgScale, 'g', -1, 64),
		"--device", m.device,
		"--out", wav}
}

// execMoss runs a MOSS command and returns its output (the runner's last
// line is a JSON report).
func execMoss(ctx context.Context, argv []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "TORCHDYNAMO_DISABLE=1", "HF_HUB_OFFLINE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := strings.TrimSpace(string(out))
		if len(tail) > 800 {
			tail = tail[len(tail)-800:]
		}
		return out, fmt.Errorf("%s: %w: %s", filepath.Base(argv[0]), err, tail)
	}
	return out, nil
}

// mossReport is the runner's JSON line.
type mossReport struct {
	Device     string  `json:"device"`
	GenSeconds float64 `json:"gen_seconds"`
	Peak       float64 `json:"peak"`
	MaxRSSGB   float64 `json:"maxrss_gb"`
}

// lastReport finds the runner's JSON report: the last line that parses.
func lastReport(out []byte) (mossReport, bool) {
	var r mossReport
	found := false
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		var try mossReport
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &try) == nil {
			r, found = try, true
		}
	}
	return r, found
}

// parseVMStat is free + inactive memory in GB from macOS vm_stat output
// (inactive pages are reclaimed before anything is paged out).
func parseVMStat(out string) (float64, error) {
	page := 4096.0
	if i := strings.Index(out, "page size of "); i >= 0 {
		f := strings.Fields(out[i+len("page size of "):])
		if len(f) > 0 {
			if v, err := strconv.ParseFloat(f[0], 64); err == nil && v > 0 {
				page = v
			}
		}
	}
	pages := map[string]float64{}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "."), 64)
		if err == nil {
			pages[strings.TrimSpace(k)] = n
		}
	}
	free, okF := pages["Pages free"]
	inactive, okI := pages["Pages inactive"]
	if !okF || !okI {
		return 0, errors.New("vm_stat output has no free/inactive page counts")
	}
	return (free + inactive) * page / 1e9, nil
}

// freeMemoryGB reads the free + inactive memory; ok is false where it
// cannot be read (only macOS's vm_stat is understood).
func freeMemoryGB() (float64, bool) {
	if runtime.GOOS != "darwin" {
		return 0, false
	}
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, false
	}
	gb, err := parseVMStat(string(out))
	return gb, err == nil
}

// waitForMemory holds a take until -moss-min-free-gb is free (a take
// peaks at 16-22 GB), polling every -moss-poll for up to -moss-max-wait.
func (a *app) waitForMemory(id string) error {
	m := a.cfg.moss
	if m.minFreeGB <= 0 {
		return nil
	}
	waited := time.Duration(0)
	for {
		gb, ok := a.freeGB()
		if !ok {
			fmt.Fprintf(a.stdout, "note: cannot read free memory here; %s goes ahead (a take needs about %g GB)\n", id, m.minFreeGB)
			return nil
		}
		if gb >= m.minFreeGB {
			return nil
		}
		if waited >= m.maxWait {
			return fmt.Errorf("%s: only %.1f GB free after waiting %s; a take needs %g GB (-moss-min-free-gb)", id, gb, waited, m.minFreeGB)
		}
		fmt.Fprintf(a.stdout, "wait: %.1f GB free, %s needs %g GB\n", gb, id, m.minFreeGB)
		a.sleep(m.poll)
		waited += m.poll
	}
}

// mossPreflight checks the runner, the Python and the model are there.
func (a *app) mossPreflight() error {
	m := a.cfg.moss
	for _, f := range []struct{ what, path, hint string }{
		{"runner script", m.runner, "-moss-runner"},
		{"MOSS Python", m.python, "-moss-python or $" + mossPythonEnv},
		{"MOSS model", filepath.Join(m.model, "model_index.json"), "-moss-model or $" + mossModelEnv},
	} {
		if _, err := os.Stat(f.path); err != nil {
			return fmt.Errorf("%s not found at %s (set %s; setup in cmd/sfxgen/main.go)", f.what, f.path, f.hint)
		}
	}
	return nil
}

// generateMoss makes each sound locally, one take at a time, logs it in
// the ledger and processes it to Ogg so -review can play it at once.
func (a *app) generateMoss(p Prompts, todo []Sound, skips []skip, l *Ledger) error {
	if a.cfg.dryRun {
		a.dryRunMoss(p, todo, skips, l)
		return nil
	}
	if err := a.mossPreflight(); err != nil {
		return err
	}
	for _, k := range skips {
		fmt.Fprintf(a.stdout, "skip %s: %s\n", k.id, k.why)
	}
	made := 0
	var failed []string
	for _, s := range todo {
		if err := a.waitForMemory(s.ID); err != nil {
			return err
		}
		n := l.nextTake(s.ID)
		seed := a.mossSeed(s.ID, n)
		raw := filepath.Join(a.cfg.out, fmt.Sprintf("%s_t%d.wav", s.ID, n))
		if err := os.MkdirAll(a.cfg.out, 0o755); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), a.cfg.moss.timeout)
		out, err := a.runMoss(ctx, a.mossCommand(p.text(s), s.Seconds, seed, raw))
		cancel()
		if err != nil {
			return fmt.Errorf("%s: %w", s.ID, err)
		}
		if _, err := os.Stat(raw); err != nil {
			return fmt.Errorf("%s: the runner wrote no %s", s.ID, raw)
		}
		l.Takes = append(l.Takes, Take{ID: s.ID, Take: n, File: a.ledgerPath(raw), Tool: mossTool, Backend: backendMoss,
			Seed: seed, Steps: a.cfg.moss.steps, Seconds: s.Seconds, Date: a.now().Format("2006-01-02"), Prompt: p.text(s)})
		if err := l.save(a.cfg.ledger); err != nil {
			return err
		}
		made++
		rep, _ := lastReport(out)
		fmt.Fprintf(a.stdout, "%s take %d: %.1f s, seed %d, made in %.0f s on %s -> %s\n",
			s.ID, n, s.Seconds, seed, rep.GenSeconds, rep.Device, a.ledgerPath(raw))
		if _, ok := events[eventOf(s.ID)]; !ok {
			continue
		}
		la, lb, err := a.processClip(raw, s.ID)
		if err != nil {
			// the take is logged; a bad one (silent, too short) is rejected and made again
			failed = append(failed, s.ID)
			fmt.Fprintf(a.stdout, "%s: processing failed: %v; -reject %s and run again\n", s.ID, err, s.ID)
			continue
		}
		fmt.Fprintf(a.stdout, "%s processed: %.1f LUFS", s.ID, la)
		if events[eventOf(s.ID)].variant {
			fmt.Fprintf(a.stdout, ", variant b %.1f LUFS", lb)
		}
		fmt.Fprintln(a.stdout)
	}
	fmt.Fprintf(a.stdout, "done: %d MOSS takes (no credits); next: -review, then -approve or -reject\n", made)
	if len(failed) > 0 {
		return fmt.Errorf("%d take(s) did not process: %s", len(failed), strings.Join(failed, ","))
	}
	return nil
}

// dryRunMoss prints the plan, the time it would take and the first
// command line; it touches nothing.
func (a *app) dryRunMoss(p Prompts, todo []Sound, skips []skip, l *Ledger) {
	secs := 0.0
	for _, s := range todo {
		fmt.Fprintf(a.stdout, "%-40s %4.1f s  seed %d\n", s.ID, s.Seconds, a.mossSeed(s.ID, l.nextTake(s.ID)))
		secs += s.Seconds
	}
	for _, k := range skips {
		fmt.Fprintf(a.stdout, "skip %s: %s\n", k.id, k.why)
	}
	fmt.Fprintf(a.stdout, "total: %d sounds, %.1f s of audio, about %d min on the local GPU (backend moss, no credits)\n",
		len(todo), secs, (len(todo)*mossClipSeconds*a.cfg.moss.steps/100+59)/60)
	if len(todo) > 0 {
		s := todo[0]
		n := l.nextTake(s.ID)
		fmt.Fprintf(a.stdout, "command: %s\n", strings.Join(a.mossCommand(p.text(s), s.Seconds, a.mossSeed(s.ID, n),
			filepath.Join(a.cfg.out, fmt.Sprintf("%s_t%d.wav", s.ID, n))), " "))
	}
	if err := a.mossPreflight(); err != nil {
		fmt.Fprintf(a.stdout, "warning: %v\n", err)
	}
}
