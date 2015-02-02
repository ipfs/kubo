package config

import "github.com/jbenet/go-ipfs/util/ipfsaddr"

// TODO replace with final servers before merge

type GCR struct {
	Servers []string
}

var DefaultGCRServers = []string{
	"/ip4/104.236.70.34/tcp/4001/QmaWJw5mcWkCThPtC7hVq28e3WbwLHnWF8HbMNJrRDybE4",
	"/ip4/128.199.72.111/tcp/4001/Qmd2cSiZUt7vhiuTmqBB7XWbkuFR8KMLiEuQssjyNXyaZT",
}

func initGCR() (*GCR, error) {
	// TODO perform validation
	return &GCR{
		Servers: DefaultGCRServers,
	}, nil
}

func (gcr *GCR) ServerIPFSAddrs() ([]ipfsaddr.IPFSAddr, error) {
	var addrs []ipfsaddr.IPFSAddr
	for _, server := range gcr.Servers {
		addr, err := ipfsaddr.ParseString(server)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}
