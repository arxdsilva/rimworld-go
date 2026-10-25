package mapgen

// validateStep is the "validate" pass (MAPGEN-2). Stub: owned by its Phase 1 agent.
type validateStep struct{}

func (validateStep) Name() string { return "validate" }

func (validateStep) Run(c *Ctx) error { return nil }
