# Assets loaded in with IPFS

This directory contains the go-ipfs assets:

* Getting started documentation (`init-doc`).
* Directory listing HTML template (`dir-index-html` git submodule).

These assets are compiled into `bindata.go` with `go generate`.

## Re-generating

Do not edit the .go files directly.

Instead, edit the source files and use `go generate` from within the
assets directory:

```
go generate .
```

## Updating dir-index-html

Upstream: https://github.com/ipfs/dir-index-html

dir-index-html is a git submodule. To update, run the following commands from
this directory.

```bash
> git -C dir-index-html pull
> git -C dir-index-html checkout vX.Y.Z # target version
```

Then, you'll need to commit the updated submodule _before_ regenerating
`bindata.go`. Otherwise, `go generate` will checkout the checked-in version of
dir-index-html.

```bash
> git add dir-index-html
> git commit -m 'chore: update dir-index-html to vX.Y.Z'
```

Finally, re-generate the directory index HTML template, tidy, and amend the previous
commit.

```bash
> go generate .
> git add bindata.go
> git add bindata_version_hash.go
> go mod tidy
> git commit --amend --no-edit

```
