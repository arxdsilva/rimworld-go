package usagehttp

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// server records what it receives and answers with a scripted status.
type server struct {
	*httptest.Server
	mu       sync.Mutex
	statuses []int
	bodies   []envelope
	raw      []string
	agents   []string
}

func newServer(t *testing.T, statuses ...int) *server {
	t.Helper()
	s := &server{statuses: statuses}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Errorf("bad body: %v", err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.bodies = append(s.bodies, env)
		s.raw = append(s.raw, string(data))
		s.agents = append(s.agents, r.UserAgent())
		status := http.StatusAccepted
		if len(s.statuses) > 0 {
			status, s.statuses = s.statuses[0], s.statuses[1:]
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) requests() []envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]envelope(nil), s.bodies...)
}

func (s *server) names() []string {
	var out []string
	for _, b := range s.requests() {
		for _, e := range b.Events {
			out = append(out, e.Name)
		}
	}
	return out
}

func newTestSender(url string) *sender {
	return newSender(senderConfig{endpoint: url, installID: "i", sessionID: "s", version: "v1", os: "darwin", arch: "arm64", now: time.Now})
}

func ev(name, tier string) event {
	return event{ID: NewID(), Name: name, Tier: tier, At: time.Now().UTC(), Props: map[string]any{}}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (s *sender) queued() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

func TestIDs(t *testing.T) {
	t.Parallel()
	a, b := NewID(), NewID()
	if a == b {
		t.Fatal("two IDs are equal")
	}
	if !ValidID(a) || a[14] != '4' {
		t.Errorf("not a version 4 UUID: %q", a)
	}
	for _, bad := range []string{"", "x", strings.ToUpper(a) + "0"} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q)", bad)
		}
	}
}

func TestFlushesOnClose(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	s := newTestSender(srv.URL)
	s.add(ev("session_start", tierBasic))
	s.close()
	reqs := srv.requests()
	if len(reqs) != 1 || len(reqs[0].Events) != 1 {
		t.Fatalf("requests = %+v", reqs)
	}
	got := reqs[0]
	if got.InstallID != "i" || got.SessionID != "s" || got.GameVersion != "v1" || got.OS != "darwin" || got.Arch != "arm64" {
		t.Errorf("envelope = %+v", got)
	}
	if srv.agents[0] != "rimworld-go/v1" {
		t.Errorf("user agent = %q", srv.agents[0])
	}
}

func TestSendsOnceEnoughEventsQueue(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	s := newTestSender(srv.URL)
	defer s.close()
	for range flushAt {
		s.add(ev("e", tierBasic))
	}
	waitFor(t, "the batch", func() bool { return len(srv.names()) == flushAt })
}

func TestBatchesStayUnderTheServerLimit(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	s := newTestSender(srv.URL)
	for range maxBatch + 5 {
		s.add(ev("e", tierBasic))
	}
	s.close()
	if got := len(srv.names()); got != maxBatch+5 {
		t.Fatalf("sent %d events", got)
	}
	for _, r := range srv.requests() {
		if len(r.Events) > maxBatch {
			t.Errorf("batch of %d", len(r.Events))
		}
	}
}

// A retried batch must carry the same IDs, or the server can't dedupe it.
func TestRetriesKeepTheEventIDs(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		srv := newServer(t, status)
		s := newTestSender(srv.URL)
		s.add(ev("e", tierBasic))
		s.kick()
		waitFor(t, "the first attempt", func() bool { return len(srv.requests()) == 1 })
		waitFor(t, "the backoff", func() bool { return s.queued() == 1 })
		s.close() // the last attempt ignores the backoff
		reqs := srv.requests()
		if len(reqs) != 2 {
			t.Fatalf("status %d: %d requests", status, len(reqs))
		}
		if reqs[0].Events[0].ID != reqs[1].Events[0].ID {
			t.Errorf("status %d: the retry changed the event ID", status)
		}
	}
}

// A request that can't be made must not bother the player: no log line, and
// the events go with the next attempt that works. Not parallel: it swaps the
// log's output, and parallel tests wait until the serial ones are done.
func TestFailedRequestsAreSilent(t *testing.T) {
	var logged bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	for name, url := range map[string]string{
		"no network":       closed.URL,
		"bad address":      "http://\x7f",
		"server refuses":   newServer(t, http.StatusBadRequest).URL,
		"server is down":   newServer(t, http.StatusInternalServerError).URL,
		"too many at once": newServer(t, http.StatusTooManyRequests).URL,
	} {
		s := newTestSender(url)
		s.add(ev("e", tierBasic))
		s.kick()
		s.close()
		if logged.Len() != 0 {
			t.Errorf("%s: logged %q", name, logged.String())
		}
	}
}

func TestEventsWaitForTheNextAttemptThatWorks(t *testing.T) {
	t.Parallel()
	srv := newServer(t, http.StatusServiceUnavailable)
	var clock atomic.Int64
	clock.Store(1000)
	s := newSender(senderConfig{endpoint: srv.URL, now: func() time.Time { return time.Unix(clock.Load(), 0) }})
	defer s.close()
	s.add(ev("first", tierBasic))
	s.kick()
	waitFor(t, "the failed attempt", func() bool { return len(srv.requests()) == 1 })
	s.add(ev("second", tierBasic))
	waitFor(t, "the next attempt", func() bool {
		clock.Add(int64(backoffMax / time.Second))
		s.kick()
		return s.queued() == 0
	})
	reqs := srv.requests()
	last := reqs[len(reqs)-1].Events
	if len(last) != 2 || last[0].Name != "first" || last[1].Name != "second" {
		t.Errorf("the next attempt carried %+v", last)
	}
}

func TestDropsBatchesTheServerRefuses(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusNotFound, http.StatusMovedPermanently} {
		srv := newServer(t, status)
		s := newTestSender(srv.URL)
		s.add(ev("e", tierBasic))
		s.close()
		if got := len(srv.requests()); got != 1 {
			t.Errorf("status %d: %d requests, want no retry", status, got)
		}
		if s.queued() != 0 {
			t.Errorf("status %d: the batch stayed queued", status)
		}
	}
}

func TestBackoffGrowsAndHonoursRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	s := &sender{cfg: senderConfig{now: func() time.Time { return now }}}
	for i, want := range []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, backoffMax, backoffMax} {
		s.backOff(0)
		if got := s.notBefore.Sub(now); got != want {
			t.Errorf("failure %d: wait %v, want %v", i+1, got, want)
		}
	}
	s = &sender{cfg: s.cfg}
	s.backOff(5 * time.Minute)
	if got := s.notBefore.Sub(now); got != 5*time.Minute {
		t.Errorf("Retry-After: wait %v", got)
	}
	s.backOff(time.Hour)
	if got := s.notBefore.Sub(now); got != backoffMax {
		t.Errorf("a huge Retry-After: wait %v, want the cap", got)
	}
}

func TestQueueDropsTheOldest(t *testing.T) {
	t.Parallel()
	s := &sender{wake: make(chan struct{}, 1)}
	for i := range maxQueue + 3 {
		s.add(event{ID: NewID(), Name: "e" + string(rune('a'+i%26)), Tier: tierBasic})
	}
	if s.queued() != maxQueue {
		t.Fatalf("queue = %d", s.queued())
	}
	if s.queue[0].Name != "e"+string(rune('a'+3)) {
		t.Errorf("oldest = %q, the first three should be gone", s.queue[0].Name)
	}
}

// A dead network must not hold up quitting the game.
func TestCloseIsBounded(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)
	s := newTestSender(srv.URL)
	s.add(ev("e", tierBasic))
	start := time.Now()
	s.close()
	if took := time.Since(start); took > closeBudget+time.Second {
		t.Errorf("close took %v", took)
	}
}

func TestCloseTwiceIsSafe(t *testing.T) {
	t.Parallel()
	s := newTestSender(newServer(t).URL)
	s.close()
	s.close()
}

// clock is a settable time source for the Tracker tests.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTracker(t *testing.T, url string, advanced bool) (*Tracker, *clock) {
	t.Helper()
	c := &clock{now: time.Unix(1_800_000_000, 0)}
	return New(Config{Endpoint: url, InstallID: NewID(), Version: "v1", OS: "linux", Arch: "amd64",
		Advanced: advanced, RAMBytes: 17 << 30, ScreenW: 1920, ScreenH: 1080, Now: c.Now}), c
}

func count(names []string, name string) (n int) {
	for _, got := range names {
		if got == name {
			n++
		}
	}
	return n
}

func TestBasicEventsAreAlwaysSent(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tr, c := newTracker(t, srv.URL, false)
	if tr.Frame(false, "high") {
		t.Error("the first frame asked for a snapshot")
	}
	c.add(heartbeatEvery)
	if tr.Frame(false, "high") {
		t.Error("a heartbeat asked for a snapshot")
	}
	tr.Colony("new", "Cassandra", "wanderer", "tundra", 250)
	tr.Snapshot(3, 4, 60)
	c.add(heartbeatEvery)
	if tr.Frame(false, "high") {
		t.Error("a snapshot is due with advanced data off")
	}
	tr.Close()

	names := srv.names()
	for want, n := range map[string]int{"session_start": 1, "session_heartbeat": 2, "session_end": 1, "hardware": 0, "colony_started": 0, "colony_snapshot": 0} {
		if got := count(names, want); got != n {
			t.Errorf("%s sent %d times, want %d (all: %v)", want, got, n, names)
		}
	}
	for _, r := range srv.requests() {
		for _, e := range r.Events {
			if e.Tier != tierBasic {
				t.Errorf("%s has tier %q", e.Name, e.Tier)
			}
		}
	}
}

func TestAdvancedEventsWhenOn(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tr, c := newTracker(t, srv.URL, true)
	tr.Frame(true, "high")
	tr.Frame(true, "high")
	tr.Colony("load", "Cassandra", "wanderer", "tundra", 250)
	c.add(snapshotEvery)
	if !tr.Frame(true, "high") {
		t.Fatal("no snapshot due")
	}
	if tr.Frame(true, "high") {
		t.Error("the next frame also asked for a snapshot")
	}
	tr.Snapshot(3, 4, 59.96)
	tr.Close()

	props := map[string]map[string]any{}
	for _, r := range srv.requests() {
		for _, e := range r.Events {
			props[e.Name] = e.Props
		}
	}
	names := srv.names()
	if count(names, "hardware") != 1 {
		t.Errorf("hardware sent %d times", count(names, "hardware"))
	}
	if p := props["hardware"]; p["ram_gib"] != 17.0 || p["screen_w"] != 1920.0 || p["quality"] != "high" {
		t.Errorf("hardware = %v", p)
	}
	if p := props["colony_started"]; p["kind"] != "load" || p["biome"] != "tundra" || p["map_size"] != 250.0 {
		t.Errorf("colony_started = %v", p)
	}
	if p := props["colony_snapshot"]; p["game_day"] != 3.0 || p["colonists"] != 4.0 || p["fps"] != 60.0 {
		t.Errorf("colony_snapshot = %v", p)
	}
}

func TestTurningAdvancedOffDropsWhatIsQueued(t *testing.T) {
	t.Parallel()
	held := newServer(t, http.StatusServiceUnavailable)
	tr, _ := newTracker(t, held.URL, true)
	tr.Frame(true, "high") // session_start, then hardware, are queued
	waitFor(t, "the first attempt", func() bool { return len(held.requests()) >= 1 })
	tr.Colony("new", "Cassandra", "wanderer", "tundra", 250)
	tr.Frame(false, "high")
	tr.Colony("new", "Cassandra", "wanderer", "tundra", 250) // ignored now
	tr.Close()

	// Only what was sent after the switch counts: the last request.
	reqs := held.requests()
	for _, e := range reqs[len(reqs)-1].Events {
		if e.Tier != tierBasic {
			t.Errorf("%s was sent after advanced data was turned off", e.Name)
		}
	}
}

func TestTurningAdvancedOnLaterSendsHardwareOnce(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tr, _ := newTracker(t, srv.URL, false)
	tr.Frame(false, "high")
	tr.Frame(true, "low")
	tr.Frame(true, "low")
	tr.Close()
	if got := count(srv.names(), "hardware"); got != 1 {
		t.Errorf("hardware sent %d times", got)
	}
}

func TestCloseBeforeTheFirstFrameSendsNothing(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tr, _ := newTracker(t, srv.URL, true)
	tr.Close()
	if names := srv.names(); len(names) != 0 {
		t.Errorf("sent %v", names)
	}
}

func TestNothingPersonalInTheRequests(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tr, _ := newTracker(t, srv.URL, true)
	tr.Frame(true, "high")
	tr.Colony("new", strings.Repeat("x", 500), "wanderer", "tundra", 250)
	tr.Close()
	all := strings.Join(srv.raw, "")
	if strings.Contains(all, strings.Repeat("x", maxPropString+1)) {
		t.Error("a long value was not clipped")
	}
	for _, field := range []string{"seed", "path", "colony_name", "pawn", "home", "ip"} {
		if strings.Contains(all, `"`+field+`"`) {
			t.Errorf("request has a %q field", field)
		}
	}
}
