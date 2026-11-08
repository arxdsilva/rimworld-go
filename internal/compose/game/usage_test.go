package game

import (
	"errors"
	"testing"

	"rimworld-go/internal/adapters/outbound/usagehttp"
	"rimworld-go/internal/platform/config"
)

func TestUsageEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, version, override string
		devRun                  bool
		want                    string
	}{
		{"release", "v1.0.0", "", false, usagehttp.Endpoint},
		{"dev build", "dev", "", false, ""},
		{"capture or probe", "v1.0.0", "", true, ""},
		{"override enables a dev build", "dev", "http://localhost:8080/e", false, "http://localhost:8080/e"},
	}
	for _, c := range cases {
		if got := usageEndpoint(c.version, c.override, c.devRun); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

type memStore struct {
	s       config.Settings
	loadErr error
	saveErr error
	saves   int
}

func (m *memStore) Load() (config.Settings, error) { return m.s, m.loadErr }
func (m *memStore) Save(s config.Settings) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.s = s
	m.saves++
	return nil
}

func TestInstallIDIsMadeOnceAndKept(t *testing.T) {
	t.Parallel()
	store := &memStore{s: config.Defaults()}
	first, ok := ensureInstallID(store)
	if !ok || !usagehttp.ValidID(first.Telemetry.InstallID) {
		t.Fatalf("first run: ok %v, id %q", ok, first.Telemetry.InstallID)
	}
	second, ok := ensureInstallID(store)
	if !ok || second.Telemetry.InstallID != first.Telemetry.InstallID {
		t.Errorf("second run: ok %v, id %q, want %q", ok, second.Telemetry.InstallID, first.Telemetry.InstallID)
	}
	if store.saves != 1 {
		t.Errorf("saved %d times, want once", store.saves)
	}
}

func TestAHandEditedInstallIDIsReplaced(t *testing.T) {
	t.Parallel()
	s := config.Defaults()
	s.Telemetry.InstallID = "my name"
	store := &memStore{s: s}
	got, ok := ensureInstallID(store)
	if !ok || !usagehttp.ValidID(got.Telemetry.InstallID) {
		t.Errorf("ok %v, id %q", ok, got.Telemetry.InstallID)
	}
}

func TestNoUsageWithoutAWorkingSettingsFile(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	if _, ok := ensureInstallID(&memStore{s: config.Defaults(), loadErr: boom}); ok {
		t.Error("an unreadable file must turn usage data off")
	}
	if _, ok := ensureInstallID(&memStore{s: config.Defaults(), saveErr: boom}); ok {
		t.Error("an unwritable file must turn usage data off")
	}
}
