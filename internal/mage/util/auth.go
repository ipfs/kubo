package util

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// https://github.com/go-git/go-git/issues/474
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

func GetSignEntity() (*openpgp.Entity, error) {
	key64 := os.Getenv("GPG_KEY")
	if key64 == "" {
		return nil, fmt.Errorf("env var GPG_KEY must be set")
	}
	key, err := base64.StdEncoding.DecodeString(key64)
	if err != nil {
		return nil, err
	}
	pass := os.Getenv("GPG_PASSPHRASE")
	bass := []byte(pass)

	list, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(key))
	if err != nil {
		return nil, err
	}
	entity := list[0]
	err = entity.PrivateKey.Decrypt(bass)
	if err != nil {
		return nil, err
	}
	for _, subkey := range entity.Subkeys {
		err = subkey.PrivateKey.Decrypt(bass)
		if err != nil {
			return nil, err
		}
	}
	return entity, nil
}
