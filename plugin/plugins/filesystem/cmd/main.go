package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/hugelgupf/p9/p9"
	"github.com/ipfs/go-ipfs/plugin/plugins/filesystem"
)

func main() {

	log.SetFlags(log.Flags() | log.Lshortfile)

	conn, err := net.Dial(filesystem.DefaultProtocol, filesystem.DefaultAddress)
	if err != nil {
		log.Fatal(err)
	}

	client, err := p9.NewClient(conn, filesystem.DefaultMSize, filesystem.DefaultVersion)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	log.Printf("Connected to server supporting version:\n%v\n\n", client.Version())

	rootRef, err := client.Attach("")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Attached to root:\n%#v\n\n", rootRef)

	readDirDBG("/", rootRef)
	readDirDBG("/ipfs", rootRef)
	readDirDBG("/ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv", rootRef)
	readDBG("/ipfs/QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB", rootRef)

	client.Close()
}

func readDirDBG(path string, fsRef p9.File) {
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(components) == 1 && components[0] == "" {
		components = nil
	}

	fmt.Printf("components: %#v\n", components)

	qids, targetRef, err := fsRef.Walk(components)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("walked to %q :\nQIDs:%v, FID:%v\n\n", path, qids, targetRef)

	refQid, ioUnit, err := targetRef.Open(0)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%q Opened:\nQID:%v, iounit:%v\n\n", path, refQid, ioUnit)

	ents, err := targetRef.Readdir(0, ^uint32(0))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("%q Readdir:\n[%d]%v\n\n", path, len(ents), ents)
	if err = targetRef.Close(); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q closed:\n%#v\n\n", path, targetRef)
}

func readDBG(path string, fsRef p9.File) {
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(components) == 1 && components[0] == "" {
		components = nil
	}

	fmt.Printf("components: %#v\n", components)

	qids, targetRef, err := fsRef.Walk(components)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("walked to %q :\nQIDs:%v, FID:%v\n\n", path, qids, targetRef)

	_, _, attr, err := targetRef.GetAttr(p9.AttrMask{Size: true})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Getattr for %q :\n%v\n\n", path, attr)

	refQid, ioUnit, err := targetRef.Open(0)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%q Opened:\nQID:%v, iounit:%v\n\n", path, refQid, ioUnit)

	buf := make([]byte, attr.Size)
	readBytes, err := targetRef.ReadAt(buf, 0)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("%q Read:\n[%d bytes]\n%s\n\n", path, readBytes, buf)
	if err = targetRef.Close(); err != nil {
		log.Fatal(err)
	}

	log.Printf("%q closed:\n%#v\n\n", path, targetRef)
}
