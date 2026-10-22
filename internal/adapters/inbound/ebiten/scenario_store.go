package ebiten

// ScenarioStore keeps custom scenario files (NG-1 §2.3, §5.7). It is
// consumer-owned: cmd/game wires the outbound scenariofile adapter.
type ScenarioStore interface {
	List() ([]ScenarioFile, error)
	Load(name string) ([]byte, error)
	Save(name string, data []byte) error
	Delete(name string) error
}

// ScenarioFile is one saved custom scenario.
type ScenarioFile struct {
	Name     string // file name without extension
	Modified string // display date
}
