package eventlog

// Loggable describes objects that can be marshalled into Metadata for logging
type Loggable interface {
	Loggable() map[string]interface{}
}

type LoggableMap map[string]interface{}

func (l LoggableMap) Loggable() map[string]interface{} {
	return l
}
