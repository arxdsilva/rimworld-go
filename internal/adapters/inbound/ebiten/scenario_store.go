package ebiten

import "rimworld-go/internal/adapters/inbound/ebiten/wizard"

// ScenarioStore keeps custom scenario files (NG-1 §2.3, §5.7). It is
// consumer-owned (by the wizard): cmd/game wires the outbound scenariofile
// adapter.
type ScenarioStore = wizard.ScenarioStore

// ScenarioFile is one saved custom scenario.
type ScenarioFile = wizard.ScenarioFile
