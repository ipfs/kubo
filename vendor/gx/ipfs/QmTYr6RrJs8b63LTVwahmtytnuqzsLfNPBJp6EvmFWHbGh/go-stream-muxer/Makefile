
godep:
	go get github.com/tools/godep

vendor: godep
	godep save -r ./...

build:
	go build ./...

test:
	go test ./...

test_race:
	go test -race -cpu 5 ./...
