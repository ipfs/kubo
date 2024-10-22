// pollEndpoint is a helper utility that waits for a http endpoint to be reachable and return with http.StatusOK
package main

import (
	"context"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var (
	host    = flag.String("host", "/ip4/127.0.0.1/tcp/5001", "the multiaddr host to dial on")
	tries   = flag.Int("tries", 10, "how many tries to make before failing")
	timeout = flag.Duration("tout", time.Second, "how long to wait between attempts")
	httpURL = flag.String("http-url", "", "HTTP URL to fetch")
	httpOut = flag.Bool("http-out", false, "Print the HTTP response body to stdout")
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

	connTries := *tries
	for connTries > 0 {
		c, err := manet.Dial(addr)
		if err == nil {
			log.Debugf("ok -  endpoint reachable with %d tries remaining, took %s", *tries, time.Since(start))
			c.Close()
			break
		}
		log.Debug("connect failed: ", err)
		time.Sleep(*timeout)
		connTries--
	}

	if err != nil {
		goto Fail
	}

	if *httpURL != "" {
		dialer := &connDialer{addr: addr}
		httpClient := http.Client{Transport: &http.Transport{
			DialContext: dialer.DialContext,
		}}
		reqTries := *tries
		for reqTries > 0 {
			try := (*tries - reqTries) + 1
			log.Debugf("trying HTTP req %d: '%s'", try, *httpURL)
			if tryHTTPGet(&httpClient, *httpURL) {
				log.Debugf("HTTP req %d to '%s' succeeded", try, *httpURL)
				goto Success
			}
			log.Debugf("HTTP req %d to '%s' failed", try, *httpURL)
			time.Sleep(*timeout)
			reqTries--
		}
		goto Fail
	}

Success:
	os.Exit(0)

Fail:
	log.Error("failed")
	os.Exit(1)
}

func tryHTTPGet(client *http.Client, url string) bool {
	resp, err := client.Get(*httpURL)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	if *httpOut {
		_, err := io.Copy(os.Stdout, resp.Body)
		if err != nil {
			panic(err)
		}
	}

	return true
}

type connDialer struct {
	addr ma.Multiaddr
}

func (d connDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&manet.Dialer{}).DialContext(ctx, d.addr)
}
