package provider

import (
	"context"
	"github.com/ipfs/go-cid"
)

// System defines the interface for interacting with the value
// provider system
type System interface {
	Run()
	Close() error
	Provide(cid.Cid) error
	Reprovide(context.Context) error
}

type system struct {
	provider   Provider
	reprovider Reprovider
}

// NewSystem constructs a new provider system from a provider and reprovider
func NewSystem(provider Provider, reprovider Reprovider) System {
	return &system{provider, reprovider}
}

// Run the provider system by running the provider and reprovider
func (s *system) Run() {
	go s.provider.Run()
	go s.reprovider.Run()
}

// Close the provider and reprovider
func (s *system) Close() error {
	var errs []error

	if err := s.provider.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := s.reprovider.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Provide a value
func (s *system) Provide(cid cid.Cid) error {
	return s.provider.Provide(cid)
}

// Reprovide all the previously provided values
func (s *system) Reprovide(ctx context.Context) error {
	return s.reprovider.Trigger(ctx)
}
