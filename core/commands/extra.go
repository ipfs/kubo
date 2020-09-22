package commands

import cmds "github.com/ipfs/go-ipfs-cmds"

func CreateCmdExtras(opts ...func(e *cmds.Extra)) *cmds.Extra {
	e := new(cmds.Extra)
	for _, o := range opts {
		o(e)
	}
	return e
}

type doesNotUseRepo struct{}

func SetDoesNotUseRepo(val bool) func(e *cmds.Extra) {
	return func(e *cmds.Extra) {
		e.SetValue(doesNotUseRepo{}, val)
	}
}

func GetDoesNotUseRepo(e *cmds.Extra) (val bool, found bool) {
	return getBoolFlag(e, doesNotUseRepo{})
}

// doesNotUseConfigAsInput describes commands that do not use the config as
// input. These commands either initialize the config or perform operations
// that don't require access to the config.
//
// pre-command hooks that require configs must not be run before these
// commands.
type doesNotUseConfigAsInput struct{}

func SetDoesNotUseConfigAsInput(val bool) func(e *cmds.Extra) {
	return func(e *cmds.Extra) {
		e.SetValue(doesNotUseConfigAsInput{}, val)
	}
}

func GetDoesNotUseConfigAsInput(e *cmds.Extra) (val bool, found bool) {
	return getBoolFlag(e, doesNotUseConfigAsInput{})
}

// preemptsAutoUpdate describes commands that must be executed without the
// auto-update pre-command hook
type preemptsAutoUpdate struct{}

func SetPreemptsAutoUpdate(val bool) func(e *cmds.Extra) {
	return func(e *cmds.Extra) {
		e.SetValue(preemptsAutoUpdate{}, val)
	}
}

func GetPreemptsAutoUpdate(e *cmds.Extra) (val bool, found bool) {
	return getBoolFlag(e, preemptsAutoUpdate{})
}

func getBoolFlag(e *cmds.Extra, key interface{}) (val bool, found bool) {
	var ival interface{}
	ival, found = e.GetValue(key)
	if !found {
		return false, false
	}
	val = ival.(bool)
	return val, found
}
