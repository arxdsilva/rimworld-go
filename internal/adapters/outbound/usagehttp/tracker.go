// Package usagehttp sends the game's usage data to the website. Basic events
// are always sent; advanced ones only while the player leaves the Options
// switch on. Nothing here touches the simulation, and nothing the player
// named (colonies, pawns, scenarios) or any file path is ever sent.
package usagehttp

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// Endpoint is where release builds send usage data.
const Endpoint = "https://arxdsilva.com/api/rimworld-go/v1/events"

const (
	heartbeatEvery = 5 * time.Minute
	snapshotEvery  = 10 * time.Minute
	maxPropString  = 64
)

// Event tiers.
const (
	tierBasic    = "basic"
	tierAdvanced = "advanced"
)

// NewID returns a random version 4 UUID.
func NewID() string { return uuid.NewString() }

// ValidID reports whether s is a UUID in the form NewID writes. uuid.Parse
// also takes urn:uuid:, braces and upper case, which NewID never writes.
func ValidID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.String() == s
}

// Config is what a Tracker needs from the composition root.
type Config struct {
	Endpoint  string
	InstallID string
	Version   string
	OS, Arch  string
	// Advanced is the Options switch as the game starts.
	Advanced bool
	// RAMBytes, ScreenW and ScreenH describe the machine for the hardware
	// event. RAM is rounded to whole GiB before it leaves the machine.
	RAMBytes         uint64
	ScreenW, ScreenH int
	// Now is the clock (nil: time.Now).
	Now func() time.Time
}

// Tracker turns what the game does into events. Its methods all run on the game
// loop, so it needs no lock, and only Close ever waits: the others just add to
// the sender's bounded queue.
type Tracker struct {
	cfg           Config
	send          *sender
	advanced      bool
	started       time.Time
	nextHeartbeat time.Time
	nextSnapshot  time.Time
	hardwareSent  bool
}

// New starts a Tracker and its sender goroutine. Call Close to stop it.
func New(cfg Config) *Tracker {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	t := &Tracker{cfg: cfg, advanced: cfg.Advanced}
	t.send = newSender(senderConfig{
		endpoint:  cfg.Endpoint,
		installID: cfg.InstallID,
		sessionID: NewID(),
		version:   cfg.Version,
		os:        cfg.OS,
		arch:      cfg.Arch,
		now:       cfg.Now,
	})
	return t
}

// Colony reports a colony starting: kind is "new" or "load".
func (t *Tracker) Colony(kind, storyteller, preset, biome string, mapSize int) {
	t.add(tierAdvanced, "colony_started", map[string]any{
		"kind":        clip(kind),
		"storyteller": clip(storyteller),
		"difficulty":  clip(preset),
		"biome":       clip(biome),
		"map_size":    mapSize,
	})
}

// Frame runs once per update and is one clock read and a few comparisons on
// nearly all of them. It returns true when a Snapshot is due; the caller
// answers with Snapshot, or skips it when no colony is running.
func (t *Tracker) Frame(advanced bool, quality string) (snapshotDue bool) {
	now := t.cfg.Now()
	if t.started.IsZero() {
		t.started = now
		t.nextHeartbeat = now.Add(heartbeatEvery)
		t.nextSnapshot = now.Add(snapshotEvery)
		t.add(tierBasic, "session_start", map[string]any{})
		t.send.kick() // a crash in the first minute still counts as a session
	}
	if advanced != t.advanced {
		t.advanced = advanced
		if !advanced {
			t.send.dropAdvanced()
		}
	}
	if advanced && !t.hardwareSent {
		t.hardwareSent = true
		t.add(tierAdvanced, "hardware", map[string]any{
			"ram_gib":  math.Round(float64(t.cfg.RAMBytes) / (1 << 30)),
			"screen_w": t.cfg.ScreenW,
			"screen_h": t.cfg.ScreenH,
			"quality":  clip(quality),
		})
	}
	if !now.Before(t.nextHeartbeat) {
		t.nextHeartbeat = now.Add(heartbeatEvery)
		t.add(tierBasic, "session_heartbeat", map[string]any{"uptime_s": seconds(now.Sub(t.started))})
	}
	if now.Before(t.nextSnapshot) {
		return false
	}
	t.nextSnapshot = now.Add(snapshotEvery)
	return advanced
}

// Snapshot reports the running colony, in answer to Frame.
func (t *Tracker) Snapshot(gameDay, colonists int, fps float64) {
	t.add(tierAdvanced, "colony_snapshot", map[string]any{
		"game_day":  gameDay,
		"colonists": colonists,
		"fps":       math.Round(fps*10) / 10,
	})
}

// Close reports the session's end, spends at most closeBudget sending what is
// queued, and stops the sender. Call it once the game loop has ended.
func (t *Tracker) Close() {
	if !t.started.IsZero() {
		t.add(tierBasic, "session_end", map[string]any{"duration_s": seconds(t.cfg.Now().Sub(t.started))})
	}
	t.send.close()
}

func (t *Tracker) add(tier, name string, props map[string]any) {
	if tier == tierAdvanced && !t.advanced {
		return
	}
	t.send.add(event{ID: NewID(), Name: name, Tier: tier, At: t.cfg.Now().UTC(), Props: props})
}

func seconds(d time.Duration) int64 { return int64(d / time.Second) }

// clip bounds a value that comes from game content, which a mod can extend.
func clip(s string) string {
	if len(s) > maxPropString {
		return s[:maxPropString]
	}
	return s
}
