test: go_test other_tests

other_tests:
	cd test && make test

go_test: go_deps
	go test -race -cpu=5 -v ./...

go_deps:
	go get code.google.com/p/go.crypto/sha3
	go get github.com/jbenet/go-base58
