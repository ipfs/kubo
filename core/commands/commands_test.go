package commands

import (
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func collectPaths(prefix string, cmd *cmds.Command, out map[string]struct{}) {
	for name, sub := range cmd.Subcommands {
		path := prefix + "/" + name
		out[path] = struct{}{}
		collectPaths(path, sub, out)
	}
}

func TestCommands(t *testing.T) {
	list := []string{
		"/add",
		"/bitswap",
		"/bitswap/ledger",
		"/bitswap/reprovide",
		"/bitswap/stat",
		"/bitswap/wantlist",
		"/block",
		"/block/get",
		"/block/put",
		"/block/rm",
		"/block/stat",
		"/bootstrap",
		"/bootstrap/add",
		"/bootstrap/add/default",
		"/bootstrap/list",
		"/bootstrap/rm",
		"/bootstrap/rm/all",
		"/cat",
		"/cid",
		"/cid/base32",
		"/cid/bases",
		"/cid/codecs",
		"/cid/format",
		"/cid/hashes",
		"/commands",
		"/commands/completion",
		"/commands/completion/bash",
		"/commands/completion/fish",
		"/commands/completion/zsh",
		"/config",
		"/config/edit",
		"/config/profile",
		"/config/profile/apply",
		"/config/replace",
		"/config/show",
		"/dag",
		"/dag/export",
		"/dag/get",
		"/dag/import",
		"/dag/put",
		"/dag/resolve",
		"/dag/stat",
		"/dht",
		"/dht/query",
		"/dht/findprovs",
		"/dht/findpeer",
		"/dht/get",
		"/dht/provide",
		"/dht/put",
		"/routing",
		"/routing/put",
		"/routing/get",
		"/routing/findpeer",
		"/routing/findprovs",
		"/routing/provide",
		"/routing/reprovide",
		"/diag",
		"/diag/cmds",
		"/diag/cmds/clear",
		"/diag/cmds/set-time",
		"/diag/profile",
		"/diag/sys",
		"/files",
		"/files/chcid",
		"/files/cp",
		"/files/flush",
		"/files/ls",
		"/files/mkdir",
		"/files/mv",
		"/files/read",
		"/files/rm",
		"/files/stat",
		"/files/write",
		"/files/chmod",
		"/files/touch",
		"/filestore",
		"/filestore/dups",
		"/filestore/ls",
		"/filestore/verify",
		"/get",
		"/id",
		"/key",
		"/key/export",
		"/key/gen",
		"/key/import",
		"/key/list",
		"/key/rename",
		"/key/rm",
		"/key/rotate",
		"/key/sign",
		"/key/verify",
		"/log",
		"/log/level",
		"/log/ls",
		"/log/tail",
		"/ls",
		"/mount",
		"/multibase",
		"/multibase/decode",
		"/multibase/encode",
		"/multibase/transcode",
		"/multibase/list",
		"/name",
		"/name/inspect",
		"/name/publish",
		"/name/pubsub",
		"/name/pubsub/cancel",
		"/name/pubsub/state",
		"/name/pubsub/subs",
		"/name/resolve",
		"/object",
		"/object/data",
		"/object/diff",
		"/object/get",
		"/object/links",
		"/object/new",
		"/object/patch",
		"/object/patch/add-link",
		"/object/patch/append-data",
		"/object/patch/rm-link",
		"/object/patch/set-data",
		"/object/put",
		"/object/stat",
		"/p2p",
		"/p2p/close",
		"/p2p/forward",
		"/p2p/listen",
		"/p2p/ls",
		"/p2p/stream",
		"/p2p/stream/close",
		"/p2p/stream/ls",
		"/pin",
		"/pin/add",
		"/pin/ls",
		"/pin/remote",
		"/pin/remote/add",
		"/pin/remote/ls",
		"/pin/remote/rm",
		"/pin/remote/service",
		"/pin/remote/service/add",
		"/pin/remote/service/ls",
		"/pin/remote/service/rm",
		"/pin/rm",
		"/pin/update",
		"/pin/verify",
		"/ping",
		"/pubsub",
		"/pubsub/ls",
		"/pubsub/peers",
		"/pubsub/pub",
		"/pubsub/sub",
		"/refs",
		"/refs/local",
		"/repo",
		"/repo/gc",
		"/repo/migrate",
		"/repo/stat",
		"/repo/verify",
		"/repo/version",
		"/repo/ls",
		"/resolve",
		"/shutdown",
		"/stats",
		"/stats/bitswap",
		"/stats/bw",
		"/stats/dht",
		"/stats/provide",
		"/stats/repo",
		"/swarm",
		"/swarm/addrs",
		"/swarm/addrs/listen",
		"/swarm/addrs/local",
		"/swarm/connect",
		"/swarm/disconnect",
		"/swarm/filters",
		"/swarm/filters/add",
		"/swarm/filters/rm",
		"/swarm/peers",
		"/swarm/peering",
		"/swarm/peering/add",
		"/swarm/peering/ls",
		"/swarm/peering/rm",
		"/swarm/resources",
		"/update",
		"/version",
		"/version/check",
		"/version/deps",
	}

	cmdSet := make(map[string]struct{})
	collectPaths("", Root, cmdSet)

	for _, path := range list {
		if _, ok := cmdSet[path]; !ok {
			t.Errorf("%q not in result", path)
		} else {
			delete(cmdSet, path)
		}
	}

	for path := range cmdSet {
		t.Errorf("%q in result but shouldn't be", path)
	}

	for _, path := range list {
		path = path[1:] // remove leading slash
		split := strings.Split(path, "/")
		sub, err := Root.Get(split)
		if err != nil {
			t.Errorf("error getting subcommand %q: %v", path, err)
		} else if sub == nil {
			t.Errorf("subcommand %q is nil even though there was no error", path)
		}
	}
}
