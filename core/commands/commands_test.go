package commands

import (
	"strings"
	"testing"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
)

func collectPaths(prefix string, cmd *cmds.Command, out map[string]struct{}) {
	for name, sub := range cmd.Subcommands {
		path := prefix + "/" + name
		out[path] = struct{}{}
		collectPaths(path, sub, out)
	}
}

func TestROCommands(t *testing.T) {
	list := []string{
		"/block",
		"/block/get",
		"/block/stat",
		"/cat",
		"/commands",
		"/dag",
		"/dag/get",
		"/dag/resolve",
		"/dns",
		"/get",
		"/ls",
		"/name",
		"/name/resolve",
		"/object",
		"/object/data",
		"/object/get",
		"/object/links",
		"/object/stat",
		"/refs",
		"/resolve",
		"/version",
	}

	cmdSet := make(map[string]struct{})
	collectPaths("", RootRO, cmdSet)

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
		sub, err := RootRO.Get(split)
		if err != nil {
			t.Errorf("error getting subcommand %q: %v", path, err)
		} else if sub == nil {
			t.Errorf("subcommand %q is nil even though there was no error", path)
		}
	}
}
func TestCommands(t *testing.T) {
	list := []string{
		"/add",
		"/bitswap",
		"/bitswap/ledger",
		"/bitswap/reprovide",
		"/bitswap/stat",
		"/bitswap/unwant",
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
		"/commands",
		"/config",
		"/config/edit",
		"/config/replace",
		"/config/show",
		"/config/profile",
		"/config/profile/apply",
		"/dag",
		"/dag/get",
		"/dag/put",
		"/dag/resolve",
		"/dht",
		"/dht/findpeer",
		"/dht/findprovs",
		"/dht/get",
		"/dht/provide",
		"/dht/put",
		"/dht/query",
		"/diag",
		"/diag/cmds",
		"/diag/cmds/clear",
		"/diag/cmds/set-time",
		"/diag/sys",
		"/dns",
		"/file",
		"/file/ls",
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
		"/filestore",
		"/filestore/dups",
		"/filestore/ls",
		"/filestore/verify",
		"/files/write",
		"/get",
		"/id",
		"/key",
		"/key/gen",
		"/key/list",
		"/key/rename",
		"/key/rm",
		"/log",
		"/log/level",
		"/log/ls",
		"/log/tail",
		"/ls",
		"/mount",
		"/name",
		"/name/publish",
		"/name/pubsub",
		"/name/pubsub/state",
		"/name/pubsub/subs",
		"/name/pubsub/cancel",
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
		"/p2p/listener",
		"/p2p/listener/close",
		"/p2p/listener/ls",
		"/p2p/listener/open",
		"/p2p/stream",
		"/p2p/stream/close",
		"/p2p/stream/dial",
		"/p2p/stream/ls",
		"/pin",
		"/pin/add",
		"/ping",
		"/pin/ls",
		"/pin/rm",
		"/pin/update",
		"/pin/verify",
		"/pubsub",
		"/pubsub/ls",
		"/pubsub/peers",
		"/pubsub/pub",
		"/pubsub/sub",
		"/refs",
		"/refs/local",
		"/repo",
		"/repo/fsck",
		"/repo/gc",
		"/repo/stat",
		"/repo/verify",
		"/repo/version",
		"/resolve",
		"/shutdown",
		"/stats",
		"/stats/bitswap",
		"/stats/bw",
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
		"/tar",
		"/tar/add",
		"/tar/cat",
		"/update",
		"/urlstore",
		"/urlstore/add",
		"/version",
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
