package iface

import (
	"context"
)

// RoutingAPI specifies the interface to the routing layer.
type RoutingAPI interface {
	// Get retrieves the best value for a given key
	Get(context.Context, string) ([]byte, error)

	// Put sets a value for a given key
	Put(ctx context.Context, key string, value []byte) error
}
