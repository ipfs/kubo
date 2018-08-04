package misc

import (
	"context"
	"strings"

	mbase "gx/ipfs/QmSbvata2WqNkqGtZNg8MR3SKwnB8iQ7vTPJgWqB8bC5kR/go-multibase"	
)


// GetCidBase gets the cid base to use from either the context or
// another cid or path
func GetCidBase(ctx context.Context, cidStr string) mbase.Encoder {
	encoder, ok := ctx.Value("cid-base").(mbase.Encoder)
	if ok {
		return encoder
	}
	defaultEncoder, _ := mbase.NewEncoder(mbase.Base58BTC)
	if cidStr != "" {
		cidStr = strings.TrimPrefix(cidStr, "/ipfs/")
		if cidStr == "" || strings.HasPrefix(cidStr, "Qm") {
			return defaultEncoder
		}
		encoder, err := mbase.NewEncoder(mbase.Encoding(cidStr[0]))
		if err != nil {
			return defaultEncoder
		}
		return encoder
	}
	return defaultEncoder
}

