package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	repo "github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	iaddr "gx/ipfs/QmNysBQN8FUSE2TKUYRFiMrn7Fiqk5RPeCz33cKaLa6syn/go-ipfs-addr"
	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	config "gx/ipfs/QmQSG7YCizeUH2bWatzp6uK9Vm3m7LA5jpxGa9QqgpNKw4/go-ipfs-config"
	mafilter "gx/ipfs/QmSMZwvs3n4GBikZ7hKzT17c3bk65FmyZo2JqtJ16swqCv/multiaddr-filter"
	inet "gx/ipfs/QmVwU7Mgwg6qaPn9XXz93ANfq1PTxcduGRzfe41Sygg4mR/go-libp2p-net"
	pstore "gx/ipfs/QmYLXCWN2myozZpx8Wx4UjrRuQuhY3YtWoMi6SHaXii6aM/go-libp2p-peerstore"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	peer "gx/ipfs/QmcZSzKEM5yDfpZbeEEZaVmaZ1zXm6JWTbrQZSB8hCVPzk/go-libp2p-peer"
	swarm "gx/ipfs/QmdjC8HtKZpEufBL1u7WxvQn78Lqq2Wk31NJS8WvFX3crB/go-libp2p-swarm"
)

type stringList struct {
	Strings []string
}

type addrMap struct {
	Addrs map[string][]string
}

var SwarmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the swarm.",
		ShortDescription: `
'ipfs swarm' is a tool to manipulate the network swarm. The swarm is the
component that opens, listens for, and maintains connections to other
ipfs peers in the internet.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"addrs":      swarmAddrsCmd,
		"connect":    swarmConnectCmd,
		"disconnect": swarmDisconnectCmd,
		"filters":    swarmFiltersCmd,
		"peers":      swarmPeersCmd,
	},
}

var swarmPeersCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List peers with open connections.",
		ShortDescription: `
'ipfs swarm peers' lists the set of peers this node is connected to.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "display all extra information"),
		cmdkit.BoolOption("streams", "Also list information about open streams for each peer"),
		cmdkit.BoolOption("latency", "Also list information about latency to each peer"),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		verbose, _, _ := req.Option("verbose").Bool()
		latency, _, _ := req.Option("latency").Bool()
		streams, _, _ := req.Option("streams").Bool()

		conns := n.PeerHost.Network().Conns()

		var out connInfos
		for _, c := range conns {
			pid := c.RemotePeer()
			addr := c.RemoteMultiaddr()

			ci := connInfo{
				Addr: addr.String(),
				Peer: pid.Pretty(),
			}

			/*
				// FIXME(steb):
							swcon, ok := c.(*swarm.Conn)
							if ok {
								ci.Muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
							}
			*/

			if verbose || latency {
				lat := n.Peerstore.LatencyEWMA(pid)
				if lat == 0 {
					ci.Latency = "n/a"
				} else {
					ci.Latency = lat.String()
				}
			}
			if verbose || streams {
				strs := c.GetStreams()

				for _, s := range strs {
					ci.Streams = append(ci.Streams, streamInfo{Protocol: string(s.Protocol())})
				}
			}
			sort.Sort(&ci)
			out.Peers = append(out.Peers, ci)
		}

		sort.Sort(&out)
		res.SetOutput(&out)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			ci, ok := v.(*connInfos)
			if !ok {
				return nil, e.TypeErr(ci, v)
			}

			buf := new(bytes.Buffer)
			pipfs := ma.ProtocolWithCode(ma.P_IPFS).Name
			for _, info := range ci.Peers {
				ids := fmt.Sprintf("/%s/%s", pipfs, info.Peer)
				if strings.HasSuffix(info.Addr, ids) {
					fmt.Fprintf(buf, "%s", info.Addr)
				} else {
					fmt.Fprintf(buf, "%s%s", info.Addr, ids)
				}
				if info.Latency != "" {
					fmt.Fprintf(buf, " %s", info.Latency)
				}
				fmt.Fprintln(buf)

				for _, s := range info.Streams {
					if s.Protocol == "" {
						s.Protocol = "<no protocol name>"
					}

					fmt.Fprintf(buf, "  %s\n", s.Protocol)
				}
			}

			return buf, nil
		},
	},
	Type: connInfos{},
}

type streamInfo struct {
	Protocol string
}

type connInfo struct {
	Addr    string
	Peer    string
	Latency string
	Muxer   string
	Streams []streamInfo
}

func (ci *connInfo) Less(i, j int) bool {
	return ci.Streams[i].Protocol < ci.Streams[j].Protocol
}

func (ci *connInfo) Len() int {
	return len(ci.Streams)
}

func (ci *connInfo) Swap(i, j int) {
	ci.Streams[i], ci.Streams[j] = ci.Streams[j], ci.Streams[i]
}

type connInfos struct {
	Peers []connInfo
}

func (ci connInfos) Less(i, j int) bool {
	return ci.Peers[i].Addr < ci.Peers[j].Addr
}

func (ci connInfos) Len() int {
	return len(ci.Peers)
}

func (ci connInfos) Swap(i, j int) {
	ci.Peers[i], ci.Peers[j] = ci.Peers[j], ci.Peers[i]
}

var swarmAddrsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List known addresses. Useful for debugging.",
		ShortDescription: `
'ipfs swarm addrs' lists all addresses this node is aware of.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"local":  swarmAddrsLocalCmd,
		"listen": swarmAddrsListenCmd,
	},
	Run: func(req cmds.Request, res cmds.Response) {

		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		addrs := make(map[string][]string)
		ps := n.PeerHost.Network().Peerstore()
		for _, p := range ps.Peers() {
			s := p.Pretty()
			for _, a := range ps.Addrs(p) {
				addrs[s] = append(addrs[s], a.String())
			}
			sort.Sort(sort.StringSlice(addrs[s]))
		}

		res.SetOutput(&addrMap{Addrs: addrs})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			m, ok := v.(*addrMap)
			if !ok {
				return nil, e.TypeErr(m, v)
			}

			// sort the ids first
			ids := make([]string, 0, len(m.Addrs))
			for p := range m.Addrs {
				ids = append(ids, p)
			}
			sort.Sort(sort.StringSlice(ids))

			buf := new(bytes.Buffer)
			for _, p := range ids {
				paddrs := m.Addrs[p]
				buf.WriteString(fmt.Sprintf("%s (%d)\n", p, len(paddrs)))
				for _, addr := range paddrs {
					buf.WriteString("\t" + addr + "\n")
				}
			}
			return buf, nil
		},
	},
	Type: addrMap{},
}

var swarmAddrsLocalCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List local addresses.",
		ShortDescription: `
'ipfs swarm addrs local' lists all local listening addresses announced to the network.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("id", "Show peer ID in addresses."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		iCtx := req.InvocContext()
		n, err := iCtx.GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		showid, _, _ := req.Option("id").Bool()
		id := n.Identity.Pretty()

		var addrs []string
		for _, addr := range n.PeerHost.Addrs() {
			saddr := addr.String()
			if showid {
				saddr = path.Join(saddr, "ipfs", id)
			}
			addrs = append(addrs, saddr)
		}
		sort.Sort(sort.StringSlice(addrs))
		res.SetOutput(&stringList{addrs})
	},
	Type: stringList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
}

var swarmAddrsListenCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List interface listening addresses.",
		ShortDescription: `
'ipfs swarm addrs listen' lists all interface addresses the node is listening on.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {

		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		var addrs []string
		maddrs, err := n.PeerHost.Network().InterfaceListenAddresses()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		for _, addr := range maddrs {
			addrs = append(addrs, addr.String())
		}
		sort.Sort(sort.StringSlice(addrs))

		res.SetOutput(&stringList{addrs})
	},
	Type: stringList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
}

var swarmConnectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Open connection to a given address.",
		ShortDescription: `
'ipfs swarm connect' opens a new direct connection to a peer address.

The address format is an IPFS multiaddr:

ipfs swarm connect /ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, true, "Address of peer to connect to.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()

		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		addrs := req.Arguments()

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		// FIXME(steb): Nasty
		swrm, ok := n.PeerHost.Network().(*swarm.Swarm)
		if !ok {
			res.SetError(fmt.Errorf("peerhost network was not swarm"), cmdkit.ErrNormal)
			return
		}

		pis, err := peersWithAddresses(addrs)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output := make([]string, len(pis))
		for i, pi := range pis {
			swrm.Backoff().Clear(pi.ID)

			output[i] = "connect " + pi.ID.Pretty()

			err := n.PeerHost.Connect(ctx, pi)
			if err != nil {
				res.SetError(fmt.Errorf("%s failure: %s", output[i], err), cmdkit.ErrNormal)
				return
			}
			output[i] += " success"
		}

		res.SetOutput(&stringList{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

var swarmDisconnectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Close connection to a given address.",
		ShortDescription: `
'ipfs swarm disconnect' closes a connection to a peer address. The address
format is an IPFS multiaddr:

ipfs swarm disconnect /ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ

The disconnect is not permanent; if ipfs needs to talk to that address later,
it will reconnect.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, true, "Address of peer to disconnect from.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		addrs := req.Arguments()

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrClient)
			return
		}

		iaddrs, err := parseAddresses(addrs)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output := make([]string, len(iaddrs))
		for i, addr := range iaddrs {
			taddr := addr.Transport()
			id := addr.ID()
			output[i] = "disconnect " + id.Pretty()

			net := n.PeerHost.Network()

			if taddr == nil {
				if net.Connectedness(id) != inet.Connected {
					output[i] += " failure: not connected"
				} else if err := net.ClosePeer(id); err != nil {
					output[i] += " failure: " + err.Error()
				} else {
					output[i] += " success"
				}
			} else {
				found := false
				for _, conn := range net.ConnsToPeer(id) {
					if !conn.RemoteMultiaddr().Equal(taddr) {
						continue
					}

					if err := conn.Close(); err != nil {
						output[i] += " failure: " + err.Error()
					} else {
						output[i] += " success"
					}
					found = true
					break
				}

				if !found {
					output[i] += " failure: conn not found"
				}
			}
		}
		res.SetOutput(&stringList{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

func stringListMarshaler(res cmds.Response) (io.Reader, error) {
	v, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	list, ok := v.(*stringList)
	if !ok {
		return nil, e.TypeErr(list, v)
	}

	buf := new(bytes.Buffer)
	for _, s := range list.Strings {
		buf.WriteString(s)
		buf.WriteString("\n")
	}

	return buf, nil
}

// parseAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns slices of multiaddrs and peerids.
func parseAddresses(addrs []string) (iaddrs []iaddr.IPFSAddr, err error) {
	iaddrs = make([]iaddr.IPFSAddr, len(addrs))
	for i, saddr := range addrs {
		iaddrs[i], err = iaddr.ParseString(saddr)
		if err != nil {
			return nil, cmds.ClientError("invalid peer address: " + err.Error())
		}
	}
	return
}

// peersWithAddresses is a function that takes in a slice of string peer addresses
// (multiaddr + peerid) and returns a slice of properly constructed peers
func peersWithAddresses(addrs []string) ([]pstore.PeerInfo, error) {
	iaddrs, err := parseAddresses(addrs)
	if err != nil {
		return nil, err
	}

	peers := make(map[peer.ID][]ma.Multiaddr, len(iaddrs))
	for _, iaddr := range iaddrs {
		id := iaddr.ID()
		current, ok := peers[id]
		if tpt := iaddr.Transport(); tpt != nil {
			peers[id] = append(current, tpt)
		} else if !ok {
			peers[id] = nil
		}
	}
	pis := make([]pstore.PeerInfo, 0, len(peers))
	for id, maddrs := range peers {
		pis = append(pis, pstore.PeerInfo{
			ID:    id,
			Addrs: maddrs,
		})
	}
	return pis, nil
}

var swarmFiltersCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manipulate address filters.",
		ShortDescription: `
'ipfs swarm filters' will list out currently applied filters. Its subcommands
can be used to add or remove said filters. Filters are specified using the
multiaddr-filter format:

Example:

    /ip4/192.168.0.0/ipcidr/16

Where the above is equivalent to the standard CIDR:

    192.168.0.0/16

Filters default to those specified under the "Swarm.AddrFilters" config key.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"add": swarmFiltersAddCmd,
		"rm":  swarmFiltersRmCmd,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrNormal)
			return
		}

		// FIXME(steb)
		swrm, ok := n.PeerHost.Network().(*swarm.Swarm)
		if !ok {
			res.SetError(errors.New("failed to cast network to swarm network"), cmdkit.ErrNormal)
			return
		}

		var output []string
		for _, f := range swrm.Filters.Filters() {
			s, err := mafilter.ConvertIPNet(f)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			output = append(output, s)
		}
		res.SetOutput(&stringList{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

var swarmFiltersAddCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Add an address filter.",
		ShortDescription: `
'ipfs swarm filters add' will add an address filter to the daemons swarm.
Filters applied this way will not persist daemon reboots, to achieve that,
add your filters to the ipfs config file.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, true, "Multiaddr to filter.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrNormal)
			return
		}

		// FIXME(steb)
		swrm, ok := n.PeerHost.Network().(*swarm.Swarm)
		if !ok {
			res.SetError(errors.New("failed to cast network to swarm network"), cmdkit.ErrNormal)
			return
		}

		if len(req.Arguments()) == 0 {
			res.SetError(errors.New("no filters to add"), cmdkit.ErrClient)
			return
		}

		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		for _, arg := range req.Arguments() {
			mask, err := mafilter.NewMask(arg)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			swrm.Filters.AddDialFilter(mask)
		}

		added, err := filtersAdd(r, cfg, req.Arguments())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return

		}

		res.SetOutput(&stringList{added})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

var swarmFiltersRmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Remove an address filter.",
		ShortDescription: `
'ipfs swarm filters rm' will remove an address filter from the daemons swarm.
Filters removed this way will not persist daemon reboots, to achieve that,
remove your filters from the ipfs config file.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, true, "Multiaddr filter to remove.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PeerHost == nil {
			res.SetError(errNotOnline, cmdkit.ErrNormal)
			return
		}

		swrm, ok := n.PeerHost.Network().(*swarm.Swarm)
		if !ok {
			res.SetError(errors.New("failed to cast network to swarm network"), cmdkit.ErrNormal)
			return
		}

		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		defer r.Close()
		cfg, err := r.Config()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if req.Arguments()[0] == "all" || req.Arguments()[0] == "*" {
			fs := swrm.Filters.Filters()
			for _, f := range fs {
				swrm.Filters.Remove(f)
			}

			removed, err := filtersRemoveAll(r, cfg)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			res.SetOutput(&stringList{removed})

			return
		}

		for _, arg := range req.Arguments() {
			mask, err := mafilter.NewMask(arg)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			swrm.Filters.Remove(mask)
		}

		removed, err := filtersRemove(r, cfg, req.Arguments())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&stringList{removed})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
	Type: stringList{},
}

func filtersAdd(r repo.Repo, cfg *config.Config, filters []string) ([]string, error) {
	addedMap := map[string]struct{}{}
	addedList := make([]string, 0, len(filters))

	// re-add cfg swarm filters to rm dupes
	oldFilters := cfg.Swarm.AddrFilters
	cfg.Swarm.AddrFilters = nil

	// add new filters
	for _, filter := range filters {
		if _, found := addedMap[filter]; found {
			continue
		}

		cfg.Swarm.AddrFilters = append(cfg.Swarm.AddrFilters, filter)
		addedList = append(addedList, filter)
		addedMap[filter] = struct{}{}
	}

	// add back original filters. in this order so that we output them.
	for _, filter := range oldFilters {
		if _, found := addedMap[filter]; found {
			continue
		}

		cfg.Swarm.AddrFilters = append(cfg.Swarm.AddrFilters, filter)
		addedMap[filter] = struct{}{}
	}

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return addedList, nil
}

func filtersRemoveAll(r repo.Repo, cfg *config.Config) ([]string, error) {
	removed := cfg.Swarm.AddrFilters
	cfg.Swarm.AddrFilters = nil

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return removed, nil
}

func filtersRemove(r repo.Repo, cfg *config.Config, toRemoveFilters []string) ([]string, error) {
	removed := make([]string, 0, len(toRemoveFilters))
	keep := make([]string, 0, len(cfg.Swarm.AddrFilters))

	oldFilters := cfg.Swarm.AddrFilters

	for _, oldFilter := range oldFilters {
		found := false
		for _, toRemoveFilter := range toRemoveFilters {
			if oldFilter == toRemoveFilter {
				found = true
				removed = append(removed, toRemoveFilter)
				break
			}
		}

		if !found {
			keep = append(keep, oldFilter)
		}
	}
	cfg.Swarm.AddrFilters = keep

	if err := r.SetConfig(cfg); err != nil {
		return nil, err
	}

	return removed, nil
}
