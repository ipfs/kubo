package autoconf

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// Backoff configuration constants for failed update retries
	backoffBaseInterval = time.Minute    // Base backoff interval (1 minute)
	backoffMaxInterval  = 24 * time.Hour // Maximum backoff interval (24 hours)
	backoffMaxMinutes   = 24 * 60        // Maximum backoff in minutes (1440 minutes = 24 hours)
)

// BackgroundUpdater handles periodic autoconf updates
type BackgroundUpdater struct {
	client          *Client
	onVersionChange func(oldVersion, newVersion int64, configURL string)
	onUpdateSuccess func(*Response)
	onUpdateError   func(error)

	// Internal state
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	mu      sync.Mutex
}

// UpdaterOption configures the background updater
type UpdaterOption func(*BackgroundUpdater) error

// NewBackgroundUpdater creates a new background updater
func NewBackgroundUpdater(client *Client, options ...UpdaterOption) (*BackgroundUpdater, error) {
	if client == nil {
		panic("autoconf: client cannot be nil")
	}

	updater := &BackgroundUpdater{
		client: client,
	}

	for _, opt := range options {
		if err := opt(updater); err != nil {
			return nil, fmt.Errorf("failed to apply updater option: %w", err)
		}
	}

	return updater, nil
}

// WithOnVersionChange sets a callback for when a new version is detected
// The callback receives oldVersion, newVersion, and configURL
func WithOnVersionChange(callback func(oldVersion, newVersion int64, configURL string)) UpdaterOption {
	return func(u *BackgroundUpdater) error {
		u.onVersionChange = callback
		return nil
	}
}

// WithOnUpdateSuccess sets a callback for successful updates
// The callback receives the Response for metadata persistence
func WithOnUpdateSuccess(callback func(*Response)) UpdaterOption {
	return func(u *BackgroundUpdater) error {
		u.onUpdateSuccess = callback
		return nil
	}
}

// WithOnUpdateError sets a callback for update errors
func WithOnUpdateError(callback func(error)) UpdaterOption {
	return func(u *BackgroundUpdater) error {
		u.onUpdateError = callback
		return nil
	}
}

// Start begins the background updater
func (u *BackgroundUpdater) Start(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.started {
		return fmt.Errorf("background updater is already started")
	}

	u.ctx, u.cancel = context.WithCancel(ctx)
	u.started = true

	u.wg.Add(1)
	go u.runUpdater()

	log.Debugf("started autoconf background updater")
	return nil
}

// Stop gracefully stops the background updater
func (u *BackgroundUpdater) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.started {
		return
	}

	u.cancel()
	u.wg.Wait()
	u.started = false

	log.Debug("stopped autoconf background updater")
}

// runUpdater is the main updater loop
func (u *BackgroundUpdater) runUpdater() {
	defer u.wg.Done()

	ticker := time.NewTicker(u.client.refreshInterval)
	defer ticker.Stop()

	failureCount := 0

	for {
		select {
		case <-u.ctx.Done():
			log.Debug("autoconf updater shutting down: context cancelled")
			return
		case <-ticker.C:
			// Attempt to update autoconf
			err := u.performUpdate()
			if err != nil {
				failureCount++
				backoff := u.calculateBackoffDelay(failureCount)

				if u.onUpdateError != nil {
					u.onUpdateError(fmt.Errorf("autoconf background update failed (attempt %d): %w, retrying in %v", failureCount, err, backoff))
				}

				// Stop regular ticker and wait for backoff
				ticker.Stop()
				select {
				case <-u.ctx.Done():
					return
				case <-time.After(backoff):
					// Reset ticker after backoff
					ticker = time.NewTicker(u.client.refreshInterval)
				}
			} else {
				// Success - reset failure count
				if failureCount > 0 {
					log.Debugf("autoconf background update succeeded after retries")
					failureCount = 0
				} else {
					log.Debug("autoconf background update succeeded")
				}
			}
		}
	}
}

// performUpdate performs a single background autoconf update
func (u *BackgroundUpdater) performUpdate() error {
	log.Debug("background update check starting")

	// Get the current cached version before fetching
	cacheDir, cacheDirErr := u.client.getCacheDir()
	var oldVersion int64 = 0
	if cacheDirErr == nil {
		oldConfig, err := u.client.getCachedConfig(cacheDir)
		if err == nil && oldConfig != nil {
			oldVersion = oldConfig.AutoConfVersion
		}
	}

	// Get config with metadata, using the client's refresh interval
	resp, err := u.client.getLatest(u.ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch autoconf: %w", err)
	}

	// Check if we got a new version and notify via callback
	if !resp.FromCache() && resp.Config.AutoConfVersion != oldVersion {
		if oldVersion == 0 {
			log.Infof("fetched autoconf version %d", resp.Config.AutoConfVersion)
		} else {
			log.Infof("fetched autoconf version %d (updated from %d)", resp.Config.AutoConfVersion, oldVersion)
		}
		if u.onVersionChange != nil {
			// Pass the selected URL that was used for this fetch
			configURL := u.client.selectURL()
			u.onVersionChange(oldVersion, resp.Config.AutoConfVersion, configURL)
		}
	}

	// Notify success callback for metadata persistence
	if u.onUpdateSuccess != nil {
		u.onUpdateSuccess(resp)
	}

	return nil
}

// calculateBackoffDelay calculates exponential backoff delay capped at 24 hours
func (u *BackgroundUpdater) calculateBackoffDelay(failureCount int) time.Duration {
	// Start with 1 minute, double each time: 1m, 2m, 4m, 8m, 16m, 32m, 1h4m, 2h8m, 4h16m, 8h32m, 17h4m
	// Cap at 24 hours
	if failureCount <= 0 {
		return backoffBaseInterval
	}

	// Calculate exponential backoff: 1 << failureCount minutes
	backoffMinutes := min(1<<failureCount, backoffMaxMinutes)

	return time.Duration(backoffMinutes) * backoffBaseInterval
}
