package usagehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	maxQueue      = 256
	flushAt       = 20
	flushEvery    = time.Minute
	maxBatch      = 50 // the server takes up to 100
	requestBudget = 5 * time.Second
	closeBudget   = 2 * time.Second
	backoffStart  = time.Minute
	backoffMax    = 15 * time.Minute
	userAgentName = "rimworld-go/"
)

// event is one entry of a request's events array.
type event struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Tier  string         `json:"tier"`
	At    time.Time      `json:"at"`
	Props map[string]any `json:"props"`
}

// envelope is a request's body.
type envelope struct {
	InstallID   string  `json:"install_id"`
	SessionID   string  `json:"session_id"`
	GameVersion string  `json:"game_version"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	Events      []event `json:"events"`
}

type senderConfig struct {
	endpoint, installID, sessionID, version, os, arch string
	now                                               func() time.Time
}

// sender owns the queue and the one goroutine that posts it. Events stay in
// the queue until the server answers, so a retry resends the same event IDs
// and the server's dedupe makes it safe. There is no disk persistence: what
// is queued when the game dies is lost, which is fine for usage data. A request
// that fails is not worth the player's attention or the log's: the sender says
// nothing and tries again with the next attempt.
type sender struct {
	cfg    senderConfig
	client *http.Client

	mu    sync.Mutex
	queue []event

	wake      chan struct{}
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	ctx       context.Context
	cancel    context.CancelFunc

	// Only the goroutine touches these.
	failures  int
	notBefore time.Time
}

func newSender(cfg senderConfig) *sender {
	ctx, cancel := context.WithCancel(context.Background())
	s := &sender{
		cfg: cfg,
		client: &http.Client{
			Timeout: requestBudget,
			// A redirect would turn the POST into a GET and lose the body.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		wake:   make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
	go s.run()
	return s
}

// add queues an event, dropping the oldest when the queue is full. It never
// blocks on the network.
func (s *sender) add(e event) {
	s.mu.Lock()
	if len(s.queue) >= maxQueue {
		s.queue = append(s.queue[:0], s.queue[1:]...)
	}
	s.queue = append(s.queue, e)
	full := len(s.queue) >= flushAt
	s.mu.Unlock()
	if full {
		s.kick()
	}
}

// kick asks the goroutine to send soon, without waiting for it.
func (s *sender) kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// dropAdvanced forgets queued advanced events: the player just turned them off.
func (s *sender) dropAdvanced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.queue[:0]
	for _, e := range s.queue {
		if e.Tier != tierAdvanced {
			kept = append(kept, e)
		}
	}
	s.queue = kept
}

// close makes one last attempt, bounded by closeBudget, and stops the
// goroutine. Quitting the game must never wait on the network for longer.
func (s *sender) close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		timer := time.NewTimer(closeBudget)
		defer timer.Stop()
		select {
		case <-s.done:
		case <-timer.C:
			s.cancel()
			<-s.done
		}
		s.cancel()
	})
}

func (s *sender) run() {
	defer close(s.done)
	ticker := time.NewTicker(flushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			s.flush() // even during a backoff: this is the last chance
			return
		case <-s.wake:
		case <-ticker.C:
		}
		if !s.cfg.now().Before(s.notBefore) {
			s.flush()
		}
	}
}

// flush posts batches until the queue is empty or the server should be left
// alone for a while.
func (s *sender) flush() {
	for {
		batch := s.next()
		if len(batch) == 0 {
			return
		}
		retryAfter, err := s.post(batch)
		if err != nil {
			s.backOff(retryAfter)
			return
		}
		s.failures = 0
		s.remove(batch)
	}
}

func (s *sender) next() []event {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := min(len(s.queue), maxBatch)
	return append([]event(nil), s.queue[:n]...)
}

// remove takes a finished batch out of the queue by ID: while it was in the
// air the queue may have overflowed or lost its advanced events.
func (s *sender) remove(batch []event) {
	gone := make(map[string]bool, len(batch))
	for _, e := range batch {
		gone[e.ID] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.queue[:0]
	for _, e := range s.queue {
		if !gone[e.ID] {
			kept = append(kept, e)
		}
	}
	s.queue = kept
}

// post sends one batch. A nil error means the batch is finished with: stored,
// or refused in a way a retry can't fix (dropped). A non-nil error keeps it
// queued for a retry after the returned wait, if the server named one.
func (s *sender) post(batch []event) (retryAfter time.Duration, err error) {
	body, err := json.Marshal(envelope{
		InstallID:   s.cfg.installID,
		SessionID:   s.cfg.sessionID,
		GameVersion: s.cfg.version,
		OS:          s.cfg.os,
		Arch:        s.cfg.arch,
		Events:      batch,
	})
	if err != nil {
		return 0, nil // a retry cannot fix this
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, s.cfg.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil // a retry cannot fix this
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgentName+s.cfg.version)
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return 0, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return parseRetryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("server answered %d", resp.StatusCode)
	default: // refused: a retry can't fix it
		return 0, nil
	}
}

// backOff doubles the wait on each failure in a row, up to backoffMax. A
// longer wait named by the server wins, within the same cap.
func (s *sender) backOff(retryAfter time.Duration) {
	wait := backoffStart << min(s.failures, 8)
	s.failures++
	wait = min(max(wait, retryAfter), backoffMax)
	s.notBefore = s.cfg.now().Add(wait)
}

func parseRetryAfter(v string) time.Duration {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}
