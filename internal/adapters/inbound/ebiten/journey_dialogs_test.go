package ebiten

// journey_dialogs_test.go: dialog and inspector pane journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_dialogs_test.go",
	[]string{"AddBill", "SetBill", "RemoveBill", "AddOperation", "RemoveOperation", "ReorderOperation", "SetSelfTend", "SetPawnBedRest", "AssignBedOwner", "SetBedMedical", "SetBedForPrisoners", "SetPrisonerMode", "ReleasePrisoner", "OpenCasket", "ProposeTrade", "CloseTrade", "CommsCallTrader", "CommsGift", "CommsRequestAid", "CommsRequestCaravan", "CloseComms", "FormCaravan", "CaravanReturn"},
	[]string{})
