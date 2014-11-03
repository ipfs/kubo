package commands

type ArgumentType int

const (
	ArgString ArgumentType = iota
	ArgFile
)

type Argument struct {
	Name               string
	Type               ArgumentType
	Required, Variadic bool
}
