// Package buildinfo holds the build's version, shown on the main menu and
// sent with update checks and usage data. Release builds set it from the
// git tag at link time:
//
//	go build -ldflags "-X rimworld-go/internal/platform/buildinfo.Version=v1.2.3" ./cmd/game
package buildinfo
