package cmdenv

import (
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

// ShouldShowProgress reports whether a progress bar should be rendered
// based on a boolean option. An explicit `--<flag>=true|false` always
// wins; when unset, it defaults to whether stderr is a terminal.
func ShouldShowProgress(req *cmds.Request, flag string) bool {
	if v, ok := req.Options[flag].(bool); ok {
		return v
	}
	return IsTerminal(os.Stderr)
}
