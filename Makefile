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

test_expensive: test_go_expensive test_sharness_expensive

test_go:
	go test -test.short ./...

test_go_expensive:
	go test ./...

test_sharness:
	cd test/ && make

test_sharness_expensive:
	cd test/ && make TEST_EXPENSIVE=1
