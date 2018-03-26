package providers

// Stat contains statistics about providers subsystem
type Stat struct {
	ProvideBufLen int
}

// Stat returns statistics about providers subsystem
func (p *providers) Stat() (*Stat, error) {
	return &Stat{
		ProvideBufLen: len(p.newBlocks),
	}, nil
}
