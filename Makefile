all:
	# no-op

godep:
	go get github.com/tools/godep

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...


install:
	cd cmd/ipfs && go install

test: test_go test_sharness

test_go:
	go test ./...

test_sharness:
	cd test/ && make
