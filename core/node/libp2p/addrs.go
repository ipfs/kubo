package libp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	mamask "github.com/whyrusleeping/multiaddr-filter"
)

func AddrFilters(filters []string) func() (*ma.Filters, Libp2pOpts, error) {
	return func() (filter *ma.Filters, opts Libp2pOpts, err error) {
		filter = ma.NewFilters()
		opts.Opts = append(opts.Opts, libp2p.ConnectionGater((*filtersConnectionGater)(filter)))
		for _, s := range filters {
			f, err := mamask.NewMask(s)
			if err != nil {
				return filter, opts, fmt.Errorf("incorrectly formatted address filter in config: %s", s)
			}
			filter.AddFilter(*f, ma.ActionDeny)
		}
		return filter, opts, nil
	}
}

func makeAddrsFactory(announce []string, appendAnnouce []string, noAnnounce []string) (p2pbhost.AddrsFactory, error) {
	var err error                     // To assign to the slice in the for loop
	existing := make(map[string]bool) // To avoid duplicates

	annAddrs := make([]ma.Multiaddr, len(announce))
	for i, addr := range announce {
		annAddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		existing[addr] = true
	}

	var appendAnnAddrs []ma.Multiaddr
	for _, addr := range appendAnnouce {
		if existing[addr] {
			// skip AppendAnnounce that is on the Announce list already
			continue
		}
		appendAddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		appendAnnAddrs = append(appendAnnAddrs, appendAddr)
	}

	filters := ma.NewFilters()
	noAnnAddrs := map[string]bool{}
	for _, addr := range noAnnounce {
		f, err := mamask.NewMask(addr)
		if err == nil {
			filters.AddFilter(*f, ma.ActionDeny)
			continue
		}
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		noAnnAddrs[string(maddr.Bytes())] = true
	}

	return func(allAddrs []ma.Multiaddr) []ma.Multiaddr {
		var addrs []ma.Multiaddr
		if len(annAddrs) > 0 {
			addrs = annAddrs
		} else {
			addrs = allAddrs
		}
		addrs = append(addrs, appendAnnAddrs...)

		var out []ma.Multiaddr
		for _, maddr := range addrs {
			// check for exact matches
			ok := noAnnAddrs[string(maddr.Bytes())]
			// check for /ipcidr matches
			if !ok && !filters.AddrBlocked(maddr) {
				out = append(out, maddr)
			}
		}
		return out
	}, nil
}

func AddrsFactory(announce []string, appendAnnouce []string, noAnnounce []string) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		addrsFactory, err := makeAddrsFactory(announce, appendAnnouce, noAnnounce)
		if err != nil {
			return opts, err
		}
		opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
		return
	}
}

func ListenOn(addresses []string) interface{} {
	return func() (opts Libp2pOpts) {
		return Libp2pOpts{
			Opts: []libp2p.Option{
				libp2p.ListenAddrStrings(addresses...),
			},
		}
	}
}
