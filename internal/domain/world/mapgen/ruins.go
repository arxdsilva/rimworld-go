package mapgen

// ruinsStep is the "ruins" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type ruinsStep struct{}

func (ruinsStep) Name() string { return "ruins" }

func (ruinsStep) Run(c *Ctx) error { return nil }
