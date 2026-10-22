package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Processing (A6): raw MP3 -> mono 48 kHz, silence trimmed from both ends,
// 8 ms / 30 ms fades, two-pass loudnorm to the event's target, Ogg Vorbis
// q4. Variant B is the same clip pitched +-4%. Deterministic for a given
// input (fixed Ogg serials, no randomness).

const (
	rate      = 48000
	truePeak  = -1.5
	fadeIn    = 0.008
	fadeOut   = 0.030
	oggQ      = "4"
	animalDir = "sfx/animals" // under assets/generated/audio
)

// trimChain removes silence below -50 dB from the start and, via a
// reversed pass, from the end. (silenceremove's stop_periods=-1 would cut
// pauses inside a clip too, which would eat the gaps of a bark or a
// chitter.)
const trimChain = "aformat=sample_rates=48000:channel_layouts=mono," +
	"silenceremove=start_periods=1:start_threshold=-50dB:detection=peak,areverse," +
	"silenceremove=start_periods=1:start_threshold=-50dB:detection=peak,areverse"

// loudness is loudnorm's print_format=json block.
type loudness struct {
	InputI      string `json:"input_i"`
	InputTP     string `json:"input_tp"`
	InputLRA    string `json:"input_lra"`
	InputThresh string `json:"input_thresh"`
	OutputI     string `json:"output_i"`
	Offset      string `json:"target_offset"`
}

// parseLoudnorm finds the JSON block loudnorm prints on stderr.
func parseLoudnorm(out string) (loudness, error) {
	var m loudness
	i := strings.LastIndex(out, "{")
	if i < 0 {
		return m, errors.New("no loudnorm measurement in ffmpeg output")
	}
	j := strings.Index(out[i:], "}")
	if j < 0 {
		return m, errors.New("truncated loudnorm measurement")
	}
	if err := json.Unmarshal([]byte(out[i:i+j+1]), &m); err != nil {
		return m, err
	}
	for _, v := range []string{m.InputI, m.InputTP, m.InputLRA, m.InputThresh, m.Offset} {
		if f, err := strconv.ParseFloat(v, 64); err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return m, fmt.Errorf("clip is silent or too short to measure (loudnorm said %q)", v)
		}
	}
	return m, nil
}

// run1 runs a tool and returns its combined output; failures carry its tail.
func run1(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 600 {
			tail = tail[len(tail)-600:]
		}
		return string(out), fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(tail))
	}
	return string(out), nil
}

func (a *app) ff(args ...string) (string, error) {
	return run1(a.ffmpeg, append([]string{"-hide_banner", "-nostdin", "-y"}, args...)...)
}

// wavSeconds is the length of a PCM WAV, read from its chunks.
func wavSeconds(file string) (float64, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var hdr [12]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil || string(hdr[:4]) != "RIFF" {
		return 0, fmt.Errorf("%s: not a WAV", file)
	}
	var bytesPerSec uint32
	for {
		var ch [8]byte
		if _, err := io.ReadFull(f, ch[:]); err != nil {
			return 0, fmt.Errorf("%s: no data chunk", file)
		}
		size := binary.LittleEndian.Uint32(ch[4:])
		switch string(ch[:4]) {
		case "fmt ":
			b := make([]byte, size)
			if _, err := io.ReadFull(f, b); err != nil || size < 16 {
				return 0, fmt.Errorf("%s: bad fmt chunk", file)
			}
			bytesPerSec = binary.LittleEndian.Uint32(b[8:])
		case "data":
			if bytesPerSec == 0 {
				return 0, fmt.Errorf("%s: data before fmt", file)
			}
			return float64(size) / float64(bytesPerSec), nil
		default:
			if _, err := f.Seek(int64(size+size&1), io.SeekCurrent); err != nil {
				return 0, err
			}
		}
	}
}

// clipFiles is where an id's Ogg files go.
func (a *app) clipFiles(id string) (aFile, bFile string) {
	dir := filepath.Join(a.cfg.assets, "generated", "audio", filepath.FromSlash(animalDir))
	return filepath.Join(dir, stemOf(id)+".ogg"), filepath.Join(dir, stemOf(id)+"_b.ogg")
}

// finish fades, normalises (two passes) and Ogg-encodes a trimmed WAV.
// It returns the loudness the file ends up at.
func (a *app) finish(in, ogg, id, variant string, target float64) (float64, error) {
	dur, err := wavSeconds(in)
	if err != nil {
		return 0, err
	}
	if dur < 0.05 {
		return 0, fmt.Errorf("clip is %.3f s after trimming (silent?)", dur)
	}
	fo := math.Min(fadeOut, dur)
	fade := fmt.Sprintf("afade=t=in:d=%g,afade=t=out:st=%.4f:d=%g", fadeIn, dur-fo, fo)
	ln := fmt.Sprintf("loudnorm=I=%g:TP=%g:LRA=11", target, truePeak)
	out, err := a.ff("-i", in, "-af", fade+","+ln+":print_format=json", "-f", "null", "-")
	if err != nil {
		return 0, err
	}
	m, err := parseLoudnorm(out)
	if err != nil {
		return 0, err
	}
	tmp, err := os.MkdirTemp("", "sfxgen")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmp)
	wav := filepath.Join(tmp, "norm.wav")
	pass2 := fmt.Sprintf("%s,%s:measured_I=%s:measured_TP=%s:measured_LRA=%s:measured_thresh=%s:offset=%s:linear=true:print_format=json,aresample=%d",
		fade, ln, m.InputI, m.InputTP, m.InputLRA, m.InputThresh, m.Offset, rate)
	out, err = a.ff("-i", in, "-af", pass2, "-ar", strconv.Itoa(rate), "-ac", "1", "-c:a", "pcm_s16le", wav)
	if err != nil {
		return 0, err
	}
	m2, err := parseLoudnorm(out)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(ogg), 0o755); err != nil {
		return 0, err
	}
	enc := filepath.Join(tmp, "out.ogg")
	if _, err := run1(a.oggenc, "-Q", "-q", oggQ, "-s", strconv.FormatUint(uint64(serialOf(id, variant)), 10), "-o", enc, wav); err != nil {
		return 0, err
	}
	if err := os.Rename(enc, ogg); err != nil {
		// different filesystem: copy
		b, rerr := os.ReadFile(enc)
		if rerr != nil {
			return 0, rerr
		}
		if err := os.WriteFile(ogg, b, 0o644); err != nil {
			return 0, err
		}
	}
	lufs, _ := strconv.ParseFloat(m2.OutputI, 64)
	return lufs, nil
}

// processClip makes id's Ogg files from a raw take.
func (a *app) processClip(raw, id string) (lufsA, lufsB float64, err error) {
	spec := events[eventOf(id)]
	tmp, err := os.MkdirTemp("", "sfxgen")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(tmp)
	trim := filepath.Join(tmp, "trim.wav")
	if _, err = a.ff("-i", raw, "-af", trimChain, "-ar", strconv.Itoa(rate), "-ac", "1", "-c:a", "pcm_f32le", trim); err != nil {
		return 0, 0, err
	}
	aFile, bFile := a.clipFiles(id)
	if lufsA, err = a.finish(trim, aFile, id, "a", spec.lufs); err != nil {
		return 0, 0, err
	}
	if !spec.variant {
		return lufsA, 0, nil
	}
	pitched := filepath.Join(tmp, "pitched.wav")
	shift := fmt.Sprintf("asetrate=%d,aresample=%d", int(math.Round(rate*variantRate(id))), rate)
	if _, err = a.ff("-i", trim, "-af", shift, "-ar", strconv.Itoa(rate), "-ac", "1", "-c:a", "pcm_f32le", pitched); err != nil {
		return 0, 0, err
	}
	lufsB, err = a.finish(pitched, bFile, id, "b", spec.lufs)
	return lufsA, lufsB, err
}

// processTakes processes the approved takes, or with -only the newest
// unrejected take of each named id (so a clip can be heard before it is
// approved).
func (a *app) processTakes() error {
	l, err := loadLedger(a.cfg.ledger)
	if err != nil {
		return err
	}
	var ids []string
	for _, id := range l.ids() {
		if a.cfg.only != "" && !matchIDs(a.cfg.only, id) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	done := 0
	for _, id := range ids {
		if _, ok := events[eventOf(id)]; !ok {
			continue
		}
		i := l.latest(id, stateApproved)
		if i < 0 && a.cfg.only != "" {
			i = l.latest(id, statePending)
		}
		if i < 0 {
			continue
		}
		raw := filepath.Join(filepath.Dir(a.cfg.ledger), filepath.FromSlash(l.Takes[i].File))
		la, lb, err := a.processClip(raw, id)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		done++
		fmt.Fprintf(a.stdout, "%s take %d: %.1f LUFS", id, l.Takes[i].Take, la)
		if events[eventOf(id)].variant {
			fmt.Fprintf(a.stdout, ", variant b %.1f LUFS", lb)
		}
		fmt.Fprintln(a.stdout)
	}
	if done == 0 {
		fmt.Fprintln(a.stdout, "process: nothing to do (no approved takes; use -only to hear pending ones)")
	}
	return nil
}

// measureLUFS is a file's integrated loudness by ffmpeg's loudnorm.
func (a *app) measureLUFS(file string) (float64, error) {
	out, err := a.ff("-i", file, "-af", "loudnorm=I=-20:TP=-1.5:LRA=11:print_format=json", "-f", "null", "-")
	if err != nil {
		return 0, err
	}
	m, err := parseLoudnorm(out)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(m.InputI, 64)
}
