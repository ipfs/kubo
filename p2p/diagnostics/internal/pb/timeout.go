package diagnostics_pb

import (
	"time"
)

func (m *Message) GetTimeoutDuration() time.Duration {
	return time.Duration(m.GetTimeout())
}

func (m *Message) SetTimeoutDuration(t time.Duration) {
	it := int64(t)
	m.Timeout = &it
}
