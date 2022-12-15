package util

// https://github.com/go-git/go-git/issues/474

import (
	"fmt"
	"net/http"
	"os"
	"encoding/base64"
)

type HeaderAuth struct {
	Key   string
	Value string
}

func (h HeaderAuth) String() string {
	return fmt.Sprintf("%s: %s", h.Key, h.Value)
}

func (h HeaderAuth) Name() string {
	return "extraheader"
}

func (h HeaderAuth) SetAuth(r *http.Request) {
	r.Header.Set(h.Key, h.Value)
}

func GetHeaderAuth() (*HeaderAuth, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("env var GITHUB_TOKEN must be set")
	}
	auth := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("pat:%s", token)))

	return &HeaderAuth{
		Key:   "Authorization",
		Value: fmt.Sprintf("Basic %s", auth),
	}, nil
}
