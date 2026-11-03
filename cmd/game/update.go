package main

import "rimworld-go/internal/adapters/outbound/updatecheck"

// updateChecker starts the check for a newer release, or returns nil when this
// run doesn't look: a dev build has nothing to compare, and captures and other
// developer runs stay off the network.
func updateChecker(buildVersion string, devRun bool) *updatecheck.Checker {
	if devRun || buildVersion == "dev" {
		return nil
	}
	return updatecheck.Start(updatecheck.Endpoint, buildVersion)
}
