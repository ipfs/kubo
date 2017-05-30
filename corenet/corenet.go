package corenet

type Corenet struct {
	Apps    AppRegistry
	Streams StreamRegistry
}

func NewCorenet() *Corenet {
	return &Corenet{}
}
