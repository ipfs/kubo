all: install

build:
	go build

install: build
	go install

# cpu profiling: `go tool pprof ipfs cpu.prof`
# mem profiling: `go tool pprof ipfs ipfs.mprof`

clean:
	rm -f cpu.prof
	rm -f ipfs.mprof
	rm -f ipfs
