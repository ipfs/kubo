default:	test

test:	*.go
	go test -v ./...

fmt:
	gofmt -w .

# Run the test in an isolated environment.
fulltest:
	docker build -t ActiveState/tail .
