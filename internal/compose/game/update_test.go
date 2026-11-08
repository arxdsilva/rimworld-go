package game

import "testing"

func TestUpdateCheckIsOffForDevBuildsAndDevRuns(t *testing.T) {
	t.Parallel()
	if updateChecker("dev", false) != nil {
		t.Error("a dev build looked for updates")
	}
	if updateChecker("v0.1.21", true) != nil {
		t.Error("a developer run looked for updates")
	}
}
