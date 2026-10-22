package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxRetries is how many times a 429/5xx/network failure is retried.
const maxRetries = 3

// fatalError is a refusal no retry can fix (bad key, plan, permission).
type fatalError struct{ msg string }

func (e fatalError) Error() string { return e.msg }

// redact removes the key from text bound for output.
func (a *app) redact(s string) string {
	if a.key == "" {
		return s
	}
	return strings.ReplaceAll(s, a.key, "[key]")
}

// detail is a short, key-free, single-line excerpt of a response body.
func (a *app) detail(body []byte) string {
	s := strings.Join(strings.Fields(a.redact(string(body))), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// do sends one authenticated request, retrying 429 and 5xx with backoff.
// Failed attempts are never takes. Errors carry the status and a body
// excerpt, never headers.
func (a *app) do(ctx context.Context, method, url string, body []byte, accept string) ([]byte, error) {
	wait := a.cfg.backoff
	var last error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
		}
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, errors.New(a.redact(err.Error()))
		}
		req.Header.Set("xi-api-key", a.key)
		req.Header.Set("Accept", accept)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := a.client.Do(req)
		if err != nil {
			last = errors.New(a.redact(err.Error()))
			continue
		}
		data, rerr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK && rerr == nil:
			return data, nil
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, fatalError{"ElevenLabs rejected the API key (HTTP 401); check " + keyEnv}
		case resp.StatusCode == http.StatusPaymentRequired:
			return nil, fatalError{"ElevenLabs refused: payment required or credits exhausted (HTTP 402): " + a.detail(data)}
		case resp.StatusCode == http.StatusForbidden:
			return nil, fatalError{"ElevenLabs forbids this request (HTTP 403): " + a.detail(data)}
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			last = fmt.Errorf("HTTP %d: %s", resp.StatusCode, a.detail(data))
		case rerr != nil:
			last = errors.New(a.redact(rerr.Error()))
		default:
			return nil, fmt.Errorf("ElevenLabs answered HTTP %d: %s", resp.StatusCode, a.detail(data))
		}
	}
	return nil, fmt.Errorf("gave up after %d retries: %w", maxRetries, last)
}

// synthesize asks for one sound effect and returns the MP3 bytes.
func (a *app) synthesize(ctx context.Context, text string, seconds, influence float64) ([]byte, error) {
	body, err := json.Marshal(map[string]any{"text": text, "duration_seconds": seconds, "prompt_influence": influence})
	if err != nil {
		return nil, err
	}
	data, err := a.do(ctx, http.MethodPost, a.cfg.baseURL+"/v1/sound-generation?output_format=mp3_44100_128", body, "audio/mpeg")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("ElevenLabs returned an empty sound")
	}
	return data, nil
}

// subscription is the credit counter (it lags the dashboard).
type subscription struct {
	Used  int `json:"character_count"`
	Limit int `json:"character_limit"`
}

func (a *app) subscription(ctx context.Context) (subscription, error) {
	var s subscription
	data, err := a.do(ctx, http.MethodGet, a.cfg.baseURL+"/v1/user/subscription", nil, "application/json")
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("subscription reply: %w", err)
	}
	if s.Limit <= 0 {
		return s, errors.New("subscription reply has no character_limit")
	}
	return s, nil
}
