package corenet

// Corenet structure holds information on currently running streams/apps
type Corenet struct {
	Apps    AppRegistry
	Streams StreamRegistry
}

// NewCorenet creates new Corenet struct
func NewCorenet() *Corenet {
	return &Corenet{}
}
