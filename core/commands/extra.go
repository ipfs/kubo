package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdutils"
)

func CreateCmdExtras(opts ...func(e *cmds.Extra)) *cmds.Extra {
	return cmdutils.CreateCmdExtras(opts...)
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

func getBoolFlag(e *cmds.Extra, key any) (val bool, found bool) {
	var ival any
	ival, found = e.GetValue(key)
	if !found {
		return false, false
	}
	val = ival.(bool)
	return val, found
}

// ResponseKind describes how a command's HTTP response should be consumed
// by the generated RPC client.
type ResponseKind = cmdutils.ResponseKind

const (
	ResponseSingle = cmdutils.ResponseSingle
	ResponseStream = cmdutils.ResponseStream
	ResponseBinary = cmdutils.ResponseBinary
)

// SetResponseKind annotates a command with its response kind for the RPC
// client generator. Use with CreateCmdExtras.
var SetResponseKind = cmdutils.SetResponseKind

// GetResponseKind returns the ResponseKind for a command. If not explicitly
// set, it infers the kind: commands with a Type field default to
// ResponseSingle, commands without default to ResponseBinary.
var GetResponseKind = cmdutils.GetResponseKind
