package commands

type ArgumentType int

const (
	ArgString ArgumentType = iota
	ArgPath
)

type Argument struct {
	Name               string
	Type               ArgumentType
	Required, Variadic bool
}
