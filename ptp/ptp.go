package ptp

// PTP structure holds information on currently running streams/apps
type PTP struct {
	Listeners ListenerRegistry
	Streams   StreamRegistry
}

// NewPTP creates new PTP struct
func NewPTP() *PTP {
	return &PTP{}
}
