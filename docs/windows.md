# Building on Windows

## Install Git For Windows

As Git is used by the Go language to download dependencies, you need
to install Git, for example from http://git-scm.com/.

You also must make sure that the directory that contains the Git For
Windows binary is in the Path environment variable. Note that Git For
Windows has a 'git' binary in a 'Git\bin' directory and another one in
a 'Git\cmd' directory. You should only put the 'Git\cmd' directory in
the Path environment variable.

## Install Go

Please install the Go language as explained on
https://golang.org/doc/install.

You must make sure that the GOROOT environment variable is set and
that the %GOROOT%/bin directory is in the Path environment variable.

The GOPATH environment variable should also be set to a directory that
you have created, and the %GOPATH/bin directory should also be in the
Path environment variable.

## Download go-ipfs and its dependencies

The following commands should download or update go-ipfs and its
dependencies:

```sh
go get -u github.com/whyrusleeping/gx
go get -u github.com/whyrusleeping/gx-go
go get -u github.com/ipfs/go-ipfs
cd %GOPATH%/src/github.com/ipfs/go-ipfs
gx --verbose install --global
```

If you get authentication problems with Git, you might want to take a
look at
https://help.github.com/articles/caching-your-github-password-in-git/
and use the suggested solution:

```sh
git config --global credential.helper wincred
```

## Build

Fuse is not supported for Windows, so you need to build IPFS without Fuse:

```sh
go build -tags nofuse ./cmd/ipfs
```

## TODO

Fix the "Build" section to pass the current commit as the Makefile does.
