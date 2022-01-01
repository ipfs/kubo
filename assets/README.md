# Assets loaded in with IPFS

This directory contains the go-ipfs assets:

* Getting started documentation (`init-doc`).
* Directory listing HTML template (`dir-index-html`).

These assets are compiled into `bindata.go` with `go generate`.

## Re-generating

Do not edit the .go files directly.

Instead, edit the source files and use `go generate` from within the
assets directory:

```
go generate .
```