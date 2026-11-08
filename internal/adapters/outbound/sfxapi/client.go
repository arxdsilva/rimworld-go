package sfxapi

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

// KeyEnv is the only place the API key may come from.
const KeyEnv = "ELEVENLABS_API_KEY"

// FatalError is a refusal no retry can fix (bad key, plan, permission).
type FatalError struct{ Msg string }

func (e FatalError) Error() string { return e.Msg }

// Client talks to the API. Key is never printed; errors redact it.
type Client struct {
	HTTP    *http.Client
	Key     string
	BaseURL string
	Backoff time.Duration // first wait before a retry; it doubles
}

// redact removes the key from text bound for output.
func (a *Client) redact(s string) string {
	if a.Key == "" {
		return s
	}
	return strings.ReplaceAll(s, a.Key, "[key]")
}

// detail is a short, key-free, single-line excerpt of a response body.
func (a *Client) detail(body []byte) string {
	s := strings.Join(strings.Fields(a.redact(string(body))), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// do sends one authenticated request, retrying 429 and 5xx with backoff.
// Failed attempts are never takes. Errors carry the status and a body
// excerpt, never headers.
func (a *Client) do(ctx context.Context, method, url string, body []byte, accept string) ([]byte, error) {
	wait := a.Backoff
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
		req.Header.Set("xi-api-key", a.Key)
		req.Header.Set("Accept", accept)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := a.HTTP.Do(req)
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
			return nil, FatalError{"ElevenLabs rejected the API key (HTTP 401); check " + KeyEnv}
		case resp.StatusCode == http.StatusPaymentRequired:
			return nil, FatalError{"ElevenLabs refused: payment required or credits exhausted (HTTP 402): " + a.detail(data)}
		case resp.StatusCode == http.StatusForbidden:
			return nil, FatalError{"ElevenLabs forbids this request (HTTP 403): " + a.detail(data)}
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

// Synthesize asks for one sound effect and returns the MP3 bytes.
func (a *Client) Synthesize(ctx context.Context, text string, seconds, influence float64) ([]byte, error) {
	body, err := json.Marshal(map[string]any{"text": text, "duration_seconds": seconds, "prompt_influence": influence})
	if err != nil {
		return nil, err
	}
	data, err := a.do(ctx, http.MethodPost, a.BaseURL+"/v1/sound-generation?output_format=mp3_44100_128", body, "audio/mpeg")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("ElevenLabs returned an empty sound")
	}
	return data, nil
}

// Subscription is the credit counter (it lags the dashboard).
type Subscription struct {
	Used  int `json:"character_count"`
	Limit int `json:"character_limit"`
}

// Subscription reads the credit counter.
func (a *Client) Subscription(ctx context.Context) (Subscription, error) {
	var s Subscription
	data, err := a.do(ctx, http.MethodGet, a.BaseURL+"/v1/user/subscription", nil, "application/json")
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
