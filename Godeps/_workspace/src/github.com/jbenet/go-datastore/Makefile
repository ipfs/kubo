build:
	go build

test: build
	go test -race -cpu=5 -v ./...

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...

deps:
	go get ./...

watch:
	-make
	@echo "[watching *.go; for recompilation]"
	# for portability, use watchmedo -- pip install watchmedo
	@watchmedo shell-command --patterns="*.go;" --recursive \
		--command='make' .

godep:
	go get github.com/tools/godep
