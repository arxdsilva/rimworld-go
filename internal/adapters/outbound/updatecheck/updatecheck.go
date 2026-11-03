// Package updatecheck asks GitHub, once per run, whether a newer release of the
// game exists, and opens the download page when the player asks. A check that
// fails says nothing: the player just hears about the release on a later run.
package updatecheck

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cli/browser"
	"golang.org/x/mod/semver"
)

const (
	// Endpoint names the newest full release: GitHub leaves out drafts and
	// prereleases.
	Endpoint = "https://api.github.com/repos/arxdsilva/rimworld-go/releases/latest"
	// pageURL is the only address the browser is ever sent to. Nothing from
	// the network reaches the command that opens it.
	pageURL = "https://github.com/arxdsilva/rimworld-go/releases/latest"

	requestBudget = 10 * time.Second
	maxResponse   = 1 << 20 // a release's JSON carries its whole changelog
)

// Checker holds what the check found. Its methods run on the game loop and
// never wait on the network.
type Checker struct {
	newer atomic.Pointer[string]
	open  func(url string) error
	done  chan struct{}
}

// Start begins the one check, for the version this build is. The goroutine
// ends with the request, within requestBudget.
func Start(endpoint, current string) *Checker {
	c := &Checker{open: browser.OpenURL, done: make(chan struct{})}
	go func() {
		defer close(c.done)
		if tag := latestTag(endpoint, current); Newer(tag, current) {
			c.newer.Store(&tag)
		}
	}()
	return c
}

// UpdateAvailable is the newer release and the page to get it from, as the
// player would type it ("" until the check finds one).
func (c *Checker) UpdateAvailable() (version, page string) {
	if tag := c.newer.Load(); tag != nil {
		return *tag, strings.TrimPrefix(pageURL, "https://")
	}
	return "", ""
}

// DownloadUpdate opens the download page in the player's browser. It runs on
// the game loop and opening can wait for the browser, so it gets its own
// goroutine. If that fails the player still has the address the notice shows.
func (c *Checker) DownloadUpdate() {
	go func() { _ = c.open(pageURL) }()
}

// latestTag asks the server for the newest release's tag ("" on any failure).
func latestTag(endpoint, current string) string {
	ctx, cancel := context.WithTimeout(context.Background(), requestBudget)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rimworld-go/"+current) // GitHub refuses requests without one
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(&release); err != nil {
		return ""
	}
	return release.TagName
}

// Newer reports that latest is a full release later than current. Anything
// that isn't a version, such as a dev build's "dev", is never newer or older.
func Newer(latest, current string) bool {
	if !fullVersion(latest) || semver.Prerelease(latest) != "" || !fullVersion(current) {
		return false
	}
	return semver.Compare(latest, current) > 0 // v0.2.0 replaces v0.2.0-rc1
}

// fullVersion refuses what semver accepts as a shorthand (v1, v1.2) and build
// metadata: a release tag is vX.Y.Z, with a suffix on a prerelease or local
// build (v0.2.0-rc1).
func fullVersion(s string) bool { return s != "" && semver.Canonical(s) == s }
