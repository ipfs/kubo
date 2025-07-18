package autoconfig

import (
	"context"
	"path/filepath"

	"github.com/ipfs/kubo"
)

// KuboClient creates a client with standard Kubo defaults
func KuboClient(repoPath string) (*Client, error) {
	cacheDir := filepath.Join(repoPath, "autoconfig")
	return NewClient(
		WithCacheDir(cacheDir),
		WithUserAgent(ipfs.GetUserAgentVersion()),
		WithCacheSize(defaultCacheSize),
		WithTimeout(defaultTimeout),
	)
}

// Get is a convenience function to get the latest config with a default client
func Get(ctx context.Context, configURL, repoPath string) (*AutoConfig, error) {
	client, err := KuboClient(repoPath)
	if err != nil {
		return nil, err
	}
	return client.GetLatest(ctx, configURL)
}
