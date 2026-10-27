package ebiten

// journey_time_test.go: time, speed and dev journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_time_test.go",
	[]string{"DevStartFire"},
	[]string{"time.pause", "time.speed1", "time.speed2", "time.speed3", "time.faster", "time.slower", "dev.ultrafast", "dev.step", "dev.actions", "dev.log", "dev.inspector"})
