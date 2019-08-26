package main

import (
	"log"

	p9client "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/client"
	logging "github.com/ipfs/go-log"
)

func main() {
	logger := logging.Logger("fs-client")
	logging.SetLogLevel("fs-client", "info")

	client, err := p9client.Dial()
	if err != nil {
		log.Fatal(err)
	}

	defer client.Close()
	logger.Infof("Connected to server supporting version:\n%v\n\n", client.Version())

	rootRef, err := client.Attach("")
	if err != nil {
		log.Fatal(err)
	}
	logger.Infof("Attached to root:\n%#v\n\n", rootRef)

	logger.Info(p9client.ReadDir("/", rootRef, 0))
	logger.Info(p9client.ReadDir("/ipfs", rootRef, 0))
	logger.Info(p9client.ReadDir("/ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv", rootRef, 0))
	//readDBG("/ipfs/QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB", rootRef)
	client.Close()
}
