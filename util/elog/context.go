package elog

import (
	"errors"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

type key int

const metadataKey key = 0

func ContextWithMetadata(ctx context.Context, l Loggable) context.Context {
	existing, err := MetadataFromContext(ctx)
	if err != nil {
		// context does not contain meta. just set the new metadata
		child := context.WithValue(ctx, metadataKey, l.Loggable())
		return child
	}

	merged := DeepMerge(existing, l.Loggable())
	child := context.WithValue(ctx, metadataKey, merged)
	return child
}

func MetadataFromContext(ctx context.Context) (Metadata, error) {
	value := ctx.Value(metadataKey)
	if value != nil {
		metadata, ok := value.(Metadata)
		if ok {
			return metadata, nil
		}
	}
	return nil, errors.New("context contains no metadata")
}
