// pollEndpoint is a helper utility that waits for a http endpoint to be reachable and return with http.StatusOK
package main

import (
	"flag"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	log "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
)

var (
	host     = flag.String("host", "/ip4/127.0.0.1/tcp/5001", "the multiaddr host to dial on")
	endpoint = flag.String("ep", "/version", "which http endpoint path to hit")
	tries    = flag.Int("tries", 10, "how many tries to make before failing")
	timeout  = flag.Duration("tout", time.Second, "how long to wait between attempts")
	verbose  = flag.Bool("v", false, "verbose logging")
)

func main() {
	flag.Parse()

	// extract address from host flag
	addr, err := ma.NewMultiaddr(*host)
	if err != nil {
		log.WithField("err", err).Fatal("NewMultiaddr() failed")
	}
	p := addr.Protocols()
	if len(p) < 2 {
		log.WithField("addr", addr).Fatal("need to protocolls in host flag.")
	}
	_, host, err := manet.DialArgs(addr)
	if err != nil {
		log.WithField("err", err).Fatal("manet.DialArgs() failed")
	}

	if *verbose { // lower log level
		log.SetLevel(log.DebugLevel)
	}

	// construct url to dial
	var u url.URL
	u.Scheme = "http"
	u.Host = host
	u.Path = *endpoint

	// show what we got
	start := time.Now()
	log.WithFields(log.Fields{
		"when":    start,
		"tries":   *tries,
		"timeout": *timeout,
		"url":     u.String(),
	}).Debug("starting")

	for *tries > 0 {

		f := log.Fields{"tries": *tries}

		resp, err := http.Get(u.String())

		if err == nil {
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				f["took"] = time.Since(start)
				log.WithFields(f).Println("status ok - endpoint reachable")
				os.Exit(0)
			}

			f["status"] = resp.Status
			log.WithFields(f).Warn("response not okay")

		} else if urlErr, ok := err.(*url.Error); ok { // expected error from http.Get()
			f["urlErr"] = urlErr

			if urlErr.Op != "Get" || urlErr.URL != *endpoint {
				f["op"] = urlErr.Op
				f["url"] = urlErr.URL
				log.WithFields(f).Error("way to funky buisness..!")
			}

			if opErr, ok := urlErr.Err.(*net.OpError); ok {
				f["opErr"] = opErr
				f["connRefused"] = opErr.Err == syscall.ECONNREFUSED
				f["temporary"] = opErr.Temporary()
				log.WithFields(f).Println("net.OpError")
			}
		} else { // unexpected error from http.Get()
			f["err"] = err
			log.WithFields(f).Error("unknown error")
		}

		time.Sleep(*timeout)
		*tries--
	}

	log.Println("failed.")
	os.Exit(1)
}
