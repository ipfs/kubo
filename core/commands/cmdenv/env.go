package cmdenv

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ipfs/kubo/commands"
	"github.com/ipfs/kubo/core"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

var log = logging.Logger("core/commands/cmdenv")

// GetNode extracts the node from the environment.
func GetNode(env interface{}) (*core.IpfsNode, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.GetNode()
}

// GetApi extracts CoreAPI instance from the environment.
func GetApi(env cmds.Environment, req *cmds.Request) (coreiface.CoreAPI, error) { //nolint
	ctx, ok := env.(*commands.Context)
	if !ok {
		return nil, fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	offline, _ := req.Options["offline"].(bool)
	if !offline {
		offline, _ = req.Options["local"].(bool)
		if offline {
			log.Errorf("Command '%s', --local is deprecated, use --offline instead", strings.Join(req.Path, " "))
		}
	}
	api, err := ctx.GetAPI()
	if err != nil {
		return nil, err
	}
	if offline {
		return api.WithOptions(options.Api.Offline(offline))
	}

	return api, nil
}

// GetConfigRoot extracts the config root from the environment
func GetConfigRoot(env cmds.Environment) (string, error) {
	ctx, ok := env.(*commands.Context)
	if !ok {
		return "", fmt.Errorf("expected env to be of type %T, got %T", ctx, env)
	}

	return ctx.ConfigRoot, nil
}

// EscNonPrint converts non-printable characters and backslash into Go escape
// sequences.  This is done to display all characters in a string, including
// those that would otherwise not be displayed or have an undesirable effect on
// the display.
func EscNonPrint(s string) string {
	if !needEscape(s) {
		return s
	}

	esc := strconv.Quote(s)
	// Remove first and last quote, and unescape quotes.
	return strings.ReplaceAll(esc[1:len(esc)-1], `\"`, `"`)
}

func needEscape(s string) bool {
	if strings.ContainsRune(s, '\\') {
		return true
	}
	for _, r := range s {
		if !strconv.IsPrint(r) {
			return true
		}
	}
	return false
}
