package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	cmds "github.com/ipfs/go-ipfs/commands"
	path "github.com/ipfs/go-ipfs/path"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

func GetEventStream(req cmds.Request) (<-chan map[string]interface{}, error) {
	addr, err := fsrepo.APIAddr(req.InvocContext().ConfigRoot)
	if err != nil {
		return nil, err
	}

	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get("http://" + host + "/api/v0/log/tail?stream-channels=true")
	if err != nil {
		return nil, err
	}

	events := make(chan map[string]interface{})
	go func() {
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)
		for {
			var obj map[string]interface{}
			err := dec.Decode(&obj)
			if err != nil {
				if err != io.EOF {
					log.Error("end of event stream error: ", err)
				}
				return
			}

			select {
			case events <- obj:
			case <-req.Context().Done():
				return
			}
		}
	}()

	return events, nil
}

func PrintDebugLog(req cmds.Request) error {
	events, err := GetEventStream(req)
	if err != nil {
		return err
	}

	write := func(f string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, f+"\n", args...)
	}

	p, err := path.ParsePath(req.Arguments()[0])
	if err != nil {
		return err
	}

	target := p.Segments()[1]

	go func() {
		interested := map[string]bool{
			target: true,
		}

		for e := range events {
			switch e["event"] {
			case "blockstore.Get":
				if interested[e["key"].(string)] {
					write(" * got block locally: %s", e["key"])
				}
			case "findProviders":
				if interested[e["key"].(string)] {
					write(" * searching for providers for %s", e["key"])
				}
			case "receivedBlock":
				if interested[e["key"].(string)] {
					write(" * got %s from %s", e["key"], e["peerID"])
				}
			case "gotProvider":
				if interested[e["key"].(string)] {
					write(" * got provider %s for %s", e["peerID"], e["key"])
				}
			case "getDAG":
				if interested[e["key"].(string)] {
					keys, ok := e["keys"].([]interface{})
					if ok {
						for _, k := range keys {
							interested[k.(string)] = true
						}
					}
				}
			case "path.ResolveLinks":
				if interested[e["key"].(string)] {
					nkey := e["nextKey"].(string)
					interested[nkey] = true
					write(" * resolve elem %q = %s", e["linkname"], nkey)
				}
			default:
				write("UNRECOGNIZED: %s", e["event"])
			}
		}
	}()

	return nil
}
