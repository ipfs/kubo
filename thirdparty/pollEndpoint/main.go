// pollEndpoint is a helper utility that waits for a http endpoint to be reachable and return with http.StatusOK
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	log "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
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
		log.WithField("addr", addr).Fatal("need two protocols in host flag (/ip/tcp)")
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

		err := checkOK(http.Get(u.String()))
		if err == nil {
			f["took"] = time.Since(start)
			log.WithFields(f).Println("status ok - endpoint reachable")
			os.Exit(0)
		}
		f["error"] = err
		log.WithFields(f).Debug("get failed")
		time.Sleep(*timeout)
		*tries--
	}

	log.Println("failed.")
	os.Exit(1)
}

func checkOK(resp *http.Response, err error) error {
	if err == nil { // request worked
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pollEndpoint: ioutil.ReadAll() Error: %s", err)
		}
		return fmt.Errorf("Response not OK. %d %s %q", resp.StatusCode, resp.Status, string(body))
	}
	return err
}
