package cmdenv

import (
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

// ProgressBarFullTemplate is the pb/v3 template used by transfer
// commands once the total byte count is known: byte counter, bar,
// speed, percent, and ETA. Explicit format args override pb's
// defaults so the rate renders as "MiB/s" (not "MiB p/s") and the
// remaining time falls back to "ETA ?" while speed is unknown.
const ProgressBarFullTemplate = `{{counters . }} {{bar . }} {{speed . "%s/s" "?/s"}} {{percent . }} {{rtime . "ETA %s" "%s" "ETA ?"}}`

// ShouldShowProgress reports whether a progress bar should be rendered
// based on a boolean option. An explicit `--<flag>=true|false` always
// wins; when unset, it defaults to whether stderr is a terminal.
func ShouldShowProgress(req *cmds.Request, flag string) bool {
	if v, ok := req.Options[flag].(bool); ok {
		return v
	}
	return IsTerminal(os.Stderr)
}
