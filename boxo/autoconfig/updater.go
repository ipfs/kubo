package autoconfig

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultUpdateInterval = 24 * time.Hour
)

// BackgroundUpdater handles periodic autoconfig updates
type BackgroundUpdater struct {
	client          *Client
	configURL       string
	updateInterval  time.Duration
	onVersionChange func(oldVersion, newVersion int64, configURL string)
	onUpdateSuccess func(*AutoConfigResponse)
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
func NewBackgroundUpdater(client *Client, configURL string, options ...UpdaterOption) (*BackgroundUpdater, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if configURL == "" {
		return nil, fmt.Errorf("configURL cannot be empty")
	}

	updater := &BackgroundUpdater{
		client:         client,
		configURL:      configURL,
		updateInterval: defaultUpdateInterval,
	}

	for _, opt := range options {
		if err := opt(updater); err != nil {
			return nil, fmt.Errorf("failed to apply updater option: %w", err)
		}
	}

	return updater, nil
}

// WithUpdateInterval sets the interval between update checks
func WithUpdateInterval(interval time.Duration) UpdaterOption {
	return func(u *BackgroundUpdater) error {
		if interval <= 0 {
			return fmt.Errorf("update interval must be positive")
		}
		u.updateInterval = interval
		return nil
	}
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
// The callback receives the AutoConfigResponse for metadata persistence
func WithOnUpdateSuccess(callback func(*AutoConfigResponse)) UpdaterOption {
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

	log.Debugf("started autoconfig background updater with interval %v", u.updateInterval)
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

	log.Debug("stopped autoconfig background updater")
}

// runUpdater is the main updater loop
func (u *BackgroundUpdater) runUpdater() {
	defer u.wg.Done()

	ticker := time.NewTicker(u.updateInterval)
	defer ticker.Stop()

	failureCount := 0

	for {
		select {
		case <-u.ctx.Done():
			log.Debug("autoconfig updater shutting down: context cancelled")
			return
		case <-ticker.C:
			// Attempt to update autoconfig
			err := u.performUpdate()
			if err != nil {
				failureCount++
				backoff := u.calculateBackoffDelay(failureCount)

				if u.onUpdateError != nil {
					u.onUpdateError(fmt.Errorf("autoconfig background update failed (attempt %d): %w, retrying in %v", failureCount, err, backoff))
				}

				// Stop regular ticker and wait for backoff
				ticker.Stop()
				select {
				case <-u.ctx.Done():
					return
				case <-time.After(backoff):
					// Reset ticker after backoff
					ticker = time.NewTicker(u.updateInterval)
				}
			} else {
				// Success - reset failure count
				if failureCount > 0 {
					log.Debugf("autoconfig background update succeeded after %d failed attempts", failureCount)
					failureCount = 0
				} else {
					log.Debug("autoconfig background update succeeded")
				}
			}
		}
	}
}

// performUpdate performs a single background autoconfig update
func (u *BackgroundUpdater) performUpdate() error {
	// Get the current cached version before fetching
	oldConfig, err := u.client.GetLatestFromCacheOnly(u.client.cacheDir)
	var oldVersion int64 = 0
	if err == nil && oldConfig != nil {
		oldVersion = oldConfig.AutoConfigVersion
	}

	// Get fresh autoconfig with metadata
	resp, err := u.client.GetLatestWithMetadata(u.ctx, u.configURL)
	if err != nil {
		return fmt.Errorf("failed to fetch autoconfig: %w", err)
	}

	// Check if we got a new version and notify via callback
	if !resp.FromCache && resp.Config.AutoConfigVersion != oldVersion {
		if u.onVersionChange != nil {
			u.onVersionChange(oldVersion, resp.Config.AutoConfigVersion, u.configURL)
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
		return time.Minute
	}

	// Calculate exponential backoff: 1 << failureCount minutes
	backoffMinutes := 1 << failureCount
	if backoffMinutes > 24*60 { // Cap at 24 hours (1440 minutes)
		backoffMinutes = 24 * 60
	}

	return time.Duration(backoffMinutes) * time.Minute
}
