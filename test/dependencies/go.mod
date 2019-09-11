module github.com/ipfs/go-ipfs/test/dependencies

go 1.13

require (
	github.com/Kubuxu/gocovmerge v0.0.0-20161216165753-7ecaa51963cd
	github.com/golangci/golangci-lint v1.18.0
	github.com/ipfs/go-cidutil v0.0.2
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/hang-fds v0.0.1
	github.com/ipfs/iptb v1.4.0
	github.com/ipfs/iptb-plugins v0.2.0
	github.com/jbenet/go-random v0.0.0-20190219211222-123a90aedc0c
	github.com/jbenet/go-random-files v0.0.0-20190219210431-31b3f20ebded
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/multiformats/go-multiaddr-net v0.0.1
	github.com/multiformats/go-multihash v0.0.7
	gotest.tools/gotestsum v0.3.5
)

// golangci-lint likes replace directives.
replace (
	github.com/ultraware/funlen => github.com/golangci/funlen v0.0.0-20190909161642-5e59b9546114
	golang.org/x/tools => github.com/golangci/tools v0.0.0-20190910062050-3540c026601b
)
