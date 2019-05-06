package provider

import (
	"context"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
)

var (
	// StrategicProvidingEnabled toggles between the original providing mechanism
	// and the new strategic providing system
	StrategicProvidingEnabled = false

	log = logging.Logger("provider")
)

// Provider announces blocks to the network
type Provider interface {
	// Run is used to begin processing the provider work
	Run()
	// Provide takes a cid and makes an attempt to announce it to the network
	Provide(cid.Cid) error
	// Close stops the provider
	Close() error
}

// Reprovider reannounces blocks to the network
type Reprovider interface {
	// Run is used to begin processing the reprovider work and waiting for reprovide triggers
	Run()
	// Trigger a reprovide
	Trigger(context.Context) error
	// Close stops the reprovider
	Close() error
}
