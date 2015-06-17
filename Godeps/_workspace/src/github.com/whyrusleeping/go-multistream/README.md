#Multistream-select router
This package implements a simple stream router for the multistream-select protocol.
The protocol is defined [here](https://github.com/jbenet/multistream).


Usage:

```go
package main

import (
	"fmt"
	ms "github.com/whyrusleeping/go-multistream"
	"io"
	"net"
)

func main() {
	mux := ms.NewMultistreamMuxer()
	mux.AddHandler("/cats", func(rwc io.ReadWriteCloser) error {
		fmt.Fprintln(rwc, "HELLO I LIKE CATS")
		return rwc.Close()
	})
	mux.AddHandler("/dogs", func(rwc io.ReadWriteCloser) error {
		fmt.Fprintln(rwc, "HELLO I LIKE DOGS")
		return rwc.Close()
	})

	list, err := net.Listen("tcp", ":8765")
	if err != nil {
		panic(err)
	}

	for {
		con, err := list.Accept()
		if err != nil {
			panic(err)
		}

		go mux.Handle(con)
	}
}
```
