all: install

godep:
	go get github.com/tools/godep

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...

install: dep
	cd multiaddr && go install

test:
	go test ./...

dep:
	cd multiaddr && go get ./...
