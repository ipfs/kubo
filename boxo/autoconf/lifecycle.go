package autoconf

import (
	"context"
	"fmt"
	"time"
)

// Start primes the cache with the latest config and starts a background updater.
// It returns the primed config for immediate use.
// The updater can be stopped either by cancelling the context or calling Stop().
func (c *Client) Start(ctx context.Context) (*Config, error) {
	// Prime cache first with a timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, DefaultTimeout)
	config := c.GetCachedOrRefresh(ctxWithTimeout)
	cancel()

	// Create background updater with standard callbacks
	updater, err := NewBackgroundUpdater(c,
		WithOnVersionChange(func(oldVersion, newVersion int64, configURL string) {
			log.Errorf("new autoconf version %d published at %s - restart to apply updates", newVersion, configURL)
		}),
		WithOnUpdateSuccess(func(resp *Response) {
			log.Debugf("updated autoconf metadata: version %s, fetch time %s", resp.Version, resp.FetchTime.Format(time.RFC3339))
		}),
		WithOnUpdateError(func(err error) {
			log.Errorf("autoconf update error: %v", err)
		}),
	)
	if err != nil {
		return config, fmt.Errorf("failed to create background updater: %w", err)
	}

	// Start the updater - it will automatically stop when context is cancelled
	if err := updater.Start(ctx); err != nil {
		return config, fmt.Errorf("failed to start background updater: %w", err)
	}

	// Store updater reference for Stop() method
	c.updaterMu.Lock()
	c.updater = updater
	c.updaterMu.Unlock()

	// Log which URLs we're checking
	if len(c.urls) == 1 {
		log.Infof("Started autoconf background updater checking %s every %s", c.urls[0], c.refreshInterval)
	} else {
		log.Infof("Started autoconf background updater checking %d URLs (load-balanced) every %s", len(c.urls), c.refreshInterval)
	}
	return config, nil
}

// Stop gracefully stops the background updater if it's running.
// This is an alternative to cancelling the context passed to Start().
// It's safe to call Stop() multiple times.
func (c *Client) Stop() {
	c.updaterMu.Lock()
	defer c.updaterMu.Unlock()

	if c.updater != nil {
		c.updater.Stop()
		c.updater = nil
		log.Infof("Stopped autoconf background updater")
	}
}
