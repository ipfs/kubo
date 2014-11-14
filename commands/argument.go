package commands

type ArgumentType int

const (
	ArgString ArgumentType = iota
	ArgFile
)

type Argument struct {
	Name          string
	Type          ArgumentType
	Required      bool
	Variadic      bool
	SupportsStdin bool
	Description   string
}

func StringArg(name string, required, variadic bool, description string) Argument {
	return Argument{
		Name:        name,
		Type:        ArgString,
		Required:    required,
		Variadic:    variadic,
		Description: description,
	}
}

func FileArg(name string, required, variadic bool, description string) Argument {
	return Argument{
		Name:        name,
		Type:        ArgFile,
		Required:    required,
		Variadic:    variadic,
		Description: description,
	}
}

func (a Argument) EnableStdin() Argument {
	a.SupportsStdin = true
	return a
}
