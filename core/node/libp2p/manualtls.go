package libp2p

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var manualTLSLog = logging.Logger("manualtls")

// manualCertCache stores loaded certificates keyed by SNI hostname, with the
// file modification time used to invalidate stale entries.
type manualCertCache struct {
	mu      sync.RWMutex
	entries map[string]manualCertEntry
}

type manualCertEntry struct {
	cert      tls.Certificate
	certMtime time.Time
	keyMtime  time.Time
}

// newManualCertCache creates a new, empty certificate cache.
func newManualCertCache() *manualCertCache {
	return &manualCertCache{entries: make(map[string]manualCertEntry)}
}

// loadManualCert reads the cert and key files for the given SNI hostname from
// repoPath, returning a cached copy if the files have not changed since the
// last load. It returns os.ErrNotExist-style errors when files are missing so
// callers can distinguish "no manual cert" from "broken manual cert".
func (c *manualCertCache) load(repoPath, sni string) (*tls.Certificate, error) {
	if sni == "" {
		return nil, nil
	}

	certPath := filepath.Join(repoPath, sni+".crt")
	keyPath := filepath.Join(repoPath, sni+".key")

	certFi, err := os.Stat(certPath)
	if err != nil {
		return nil, err // wraps os.ErrNotExist when file is absent
	}
	keyFi, err := os.Stat(keyPath)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	if entry, ok := c.entries[sni]; ok && entry.certMtime.Equal(certFi.ModTime()) && entry.keyMtime.Equal(keyFi.ModTime()) {
		c.mu.RUnlock()
		return &entry.cert, nil
	}
	c.mu.RUnlock()

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[sni] = manualCertEntry{
		cert:      cert,
		certMtime: certFi.ModTime(),
		keyMtime:  keyFi.ModTime(),
	}
	c.mu.Unlock()

	return &cert, nil
}

// manualCertGetter returns a GetCertificate callback that serves manual TLS
// certificates from $IPFS_PATH/<sni>.crt and $IPFS_PATH/<sni>.key when present,
// falling back to the provided fallback (e.g. p2p-forge's certmagic) otherwise.
//
// This lets an operator use WSS on a custom domain (e.g. example.net) by
// placing certificate files in the IPFS repo, while AutoTLS continues to
// handle *.libp2p.direct domains.
func manualCertGetter(repoPath string, cache *manualCertCache, fallback func(*tls.ClientHelloInfo) (*tls.Certificate, error)) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert, err := cache.load(repoPath, hello.ServerName)
		if err == nil && cert != nil {
			return cert, nil
		}
		if err != nil && !os.IsNotExist(err) {
			manualTLSLog.Errorf("loading manual cert for %q: %v", hello.ServerName, err)
		}
		if fallback != nil {
			return fallback(hello)
		}
		return nil, err
	}
}
