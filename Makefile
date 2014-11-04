all:
	# no-op

godep:
	go get github.com/tools/godep

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...

# TODO remove ipfs2 once new command refactoring is complete
install:
	cd cmd/ipfs && go install
	cd cmd/ipfs2 && go install
