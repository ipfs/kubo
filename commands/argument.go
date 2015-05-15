package commands

type ArgumentType int

const (
	ArgString ArgumentType = iota
	ArgFile
	ArgStream
)

type Argument struct {
	Name          string
	Type          ArgumentType
	Required      bool // error if no value is specified
	Variadic      bool // unlimited values can be specfied
	SupportsStdin bool // can accept stdin as a value
	Recursive     bool // supports recursive file adding (with '-r' flag)
	Description   string
}

// Regular string arugment passed to the command
func StringArg(name string, required, variadic bool, description string) Argument {
	return Argument{
		Name:        name,
		Type:        ArgString,
		Required:    required,
		Variadic:    variadic,
		Description: description,
	}
}

// Opens the argument name as a file, supports EnableStdin and EnableRecursive
// EnableStdin will use stdin as the file source only if it's not from the terminal
func FileArg(name string, required, variadic bool, description string) Argument {
	return Argument{
		Name:        name,
		Type:        ArgFile,
		Required:    required,
		Variadic:    variadic,
		Description: description,
	}
}

// Used for piping to a command, differs from FileArg with the SupportsStdin set to true
// by making sure we always capture stdin.
// FileArg will only use stdin if the input is not from the keyboard (terminal).
func StreamArg(name string, required, variadic bool, description string) Argument {
	return Argument{
		Name:        name,
		Type:        ArgStream,
		Required:    required,
		Variadic:    variadic,
		Description: description,
		SupportsStdin: true,       // Always
	}
}

// TODO: modifiers might need a different API?
//       e.g. passing enum values into arg constructors variadically
//       (`FileArg("file", ArgRequired, ArgStdin, ArgRecursive)`)
func (a Argument) EnableStdin() Argument {
	a.SupportsStdin = true
	return a
}

func (a Argument) EnableRecursive() Argument {
	if a.Type != ArgFile {
		panic("Only ArgFile arguments can enable recursive")
	}

	a.Recursive = true
	return a
}
