// pollEndpoint is a helper utility that waits for a http endpoint to be reachable and return with http.StatusOK
package main

import (
	"flag"
	"os"
	"time"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var (
	host    = flag.String("host", "/ip4/127.0.0.1/tcp/5001", "the multiaddr host to dial on")
	tries   = flag.Int("tries", 10, "how many tries to make before failing")
	timeout = flag.Duration("tout", time.Second, "how long to wait between attempts")
	verbose = flag.Bool("v", false, "verbose logging")
)

var log = logging.Logger("pollEndpoint")

func main() {
	flag.Parse()

	// extract address from host flag
	addr, err := ma.NewMultiaddr(*host)
	if err != nil {
		log.Fatal("NewMultiaddr() failed: ", err)
	}

	if *verbose { // lower log level
		logging.SetDebugLogging()
	}

	// show what we got
	start := time.Now()
	log.Debugf("starting at %s, tries: %d, timeout: %s, addr: %s", start, *tries, *timeout, addr)

	for *tries > 0 {
		c, err := manet.Dial(addr)
		if err == nil {
			log.Debugf("ok -  endpoint reachable with %d tries remaining, took %s", *tries, time.Since(start))
			c.Close()
			os.Exit(0)
		}
		log.Debug("connect failed: ", err)
		time.Sleep(*timeout)
		*tries--
	}

	log.Error("failed.")
	os.Exit(1)
}
