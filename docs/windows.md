# Building on Windows

## Build

Fuse is not supported for Windows, so you need to build IPFS without Fuse:

```sh
go build -tags nofuse ./cmd/ipfs
```

## TODO

Add more instructions on setting up the golang environment on Windows
