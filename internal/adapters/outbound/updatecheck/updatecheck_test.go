package updatecheck

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.1.21", "v0.1.20", true},
		{"v0.1.21", "v0.1.21", false},
		{"v0.1.20", "v0.1.21", false},
		{"v0.2.0", "v0.1.99", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.1.10", "v0.1.9", true}, // numbers, not text
		{"v0.2.0", "v0.2.0-rc1", true},
		{"v0.1.21", "v0.2.0-rc1", false},
		{"v0.3.0-rc1", "v0.2.0", false}, // a prerelease is never offered
		{"v0.1.21", "dev", false},
		{"", "v0.1.20", false},
		{"latest", "v0.1.20", false},
		{"v99999999999999999999.0.0", "v0.1.20", false},
		{"0.1.21", "v0.1.20", false},
		{"v0.1.21/../../evil", "v0.1.20", false},
	} {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

// answer is a server with one fixed reply.
func answer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.UserAgent(), "rimworld-go/") {
			t.Errorf("request %s, agent %q", r.Method, r.UserAgent())
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFindsANewerRelease(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(map[string]any{"tag_name": "v0.1.21", "body": strings.Repeat("changes ", 1000)})
	c := Start(answer(t, http.StatusOK, string(body)).URL, "v0.1.20")
	<-c.done
	version, page := c.UpdateAvailable()
	if version != "v0.1.21" {
		t.Errorf("version %q", version)
	}
	if page != "github.com/arxdsilva/rimworld-go/releases/latest" {
		t.Errorf("page %q", page)
	}
}

func TestNothingToOfferOrAFailedCheckStaysQuiet(t *testing.T) {
	t.Parallel()
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	for name, endpoint := range map[string]string{
		"up to date":  answer(t, http.StatusOK, `{"tag_name":"v0.1.20"}`).URL,
		"older":       answer(t, http.StatusOK, `{"tag_name":"v0.1.19"}`).URL,
		"rate limit":  answer(t, http.StatusForbidden, `{"tag_name":"v9.0.0"}`).URL,
		"no release":  answer(t, http.StatusNotFound, `{}`).URL,
		"not JSON":    answer(t, http.StatusOK, `<html>`).URL,
		"no network":  closed.URL,
		"bad address": "http://\x7f",
	} {
		c := Start(endpoint, "v0.1.20")
		<-c.done
		if version, page := c.UpdateAvailable(); version != "" || page != "" {
			t.Errorf("%s: offered %q at %q", name, version, page)
		}
	}
}

func TestNothingIsOfferedBeforeTheAnswer(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.21"}`))
	}))
	t.Cleanup(srv.Close)
	c := Start(srv.URL, "v0.1.20")
	if version, _ := c.UpdateAvailable(); version != "" {
		t.Errorf("offered %q before the server answered", version)
	}
	close(release)
	<-c.done
	if version, _ := c.UpdateAvailable(); version != "v0.1.21" {
		t.Errorf("version %q after the answer", version)
	}
}

func TestDownloadOpensTheFixedPage(t *testing.T) {
	t.Parallel()
	var opened []string
	c := &Checker{open: func(url string) error {
		opened = append(opened, url)
		return errors.New("no browser") // the notice still shows the address
	}}
	c.DownloadUpdate()
	if !slices.Equal(opened, []string{pageURL}) {
		t.Errorf("opened %v", opened)
	}
}

func TestBrowserCommand(t *testing.T) {
	t.Parallel()
	const url = "https://example.test/x"
	for goos, want := range map[string][]string{
		"darwin":  {"open", url},
		"linux":   {"xdg-open", url},
		"freebsd": {"xdg-open", url},
		"windows": {"rundll32", "url.dll,FileProtocolHandler", url},
	} {
		name, args := browserCommand(goos, url)
		if got := append([]string{name}, args...); !slices.Equal(got, want) {
			t.Errorf("%s: %v", goos, got)
		}
	}
}
