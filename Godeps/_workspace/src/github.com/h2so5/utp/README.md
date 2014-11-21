utp
===

μTP (Micro Transport Protocol) implementation

[![Build status](https://ci.appveyor.com/api/projects/status/j1be8y7p6nd2wqqw?svg=true)](https://ci.appveyor.com/project/h2so5/utp)
[![Build Status](https://travis-ci.org/h2so5/utp.svg)](https://travis-ci.org/h2so5/utp)
[![GoDoc](https://godoc.org/github.com/h2so5/utp?status.svg)](http://godoc.org/github.com/h2so5/utp)

http://www.bittorrent.org/beps/bep_0029.html

**warning: This is a buggy alpha version.**

## Benchmark History

[![Benchmark status](http://107.170.244.57:80/go-utp-bench.php)]()

## Installation

```
go get github.com/h2so5/utp
```

## Example

Echo server

```go
package main

import (
	"time"

	"github.com/h2so5/utp"
)

func main() {
	ln, _ := utp.Listen("utp", ":11000")
	defer ln.Close()

	conn, _ := ln.AcceptUTP()
	conn.SetKeepAlive(time.Minute)
	defer conn.Close()
	
	for {
		var buf [1024]byte
		l, err := conn.Read(buf[:])
		if err != nil {
			break
		}
		_, err = conn.Write(buf[:l])
		if err != nil {
			break
		}
	}
}
```
