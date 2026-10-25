package mapgen

// startStep is the "start" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type startStep struct{}

func (startStep) Name() string { return "start" }

func (startStep) Run(c *Ctx) error { return nil }
