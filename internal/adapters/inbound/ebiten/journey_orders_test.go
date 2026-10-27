package ebiten

// journey_orders_test.go: orders, draft and formation journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_orders_test.go",
	[]string{"DraftPawn", "MovePawn", "Prioritize", "ClearOrders", "Capture", "Arrest", "EmergencyTend"},
	[]string{"thing.draft", "gizmo.1", "gizmo.2", "gizmo.3", "gizmo.4", "gizmo.5", "gizmo.6", "gizmo.8", "gizmo.9", "gizmo.10", "gizmo.11", "gizmo.12"})
