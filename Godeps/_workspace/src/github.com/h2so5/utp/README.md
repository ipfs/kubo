utp
===

μTP (Micro Transport Protocol) implementation

[![Build status](https://ci.appveyor.com/api/projects/status/j1be8y7p6nd2wqqw?svg=true&branch=master)](https://ci.appveyor.com/project/h2so5/utp)
[![Build Status](https://travis-ci.org/h2so5/utp.svg?branch=master)](https://travis-ci.org/h2so5/utp)
[![GoDoc](https://godoc.org/github.com/h2so5/utp?status.svg)](http://godoc.org/github.com/h2so5/utp)

http://www.bittorrent.org/beps/bep_0029.html

## Installation

```
go get github.com/h2so5/utp
```

## Debug Log

Use GO_UTP_LOGGING to show debug logs.

```
GO_UTP_LOGGING=0 go test  <- default, no logging
GO_UTP_LOGGING=1 go test
GO_UTP_LOGGING=2 go test
GO_UTP_LOGGING=3 go test
GO_UTP_LOGGING=4 go test  <- most verbose
```
