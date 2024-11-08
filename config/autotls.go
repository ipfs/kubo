package config

import p2pforge "github.com/ipshipyard/p2p-forge/client"

// AutoTLS includes optional configuration of p2p-forge client of service
// for obtaining a domain and TLS certificate to improve connectivity for web
// browser clients. More: https://github.com/ipshipyard/p2p-forge#readme
type AutoTLS struct {
	// Enables the p2p-forge feature
	Enabled Flag `json:",omitempty"`

	// Optional override of the parent domain that will be used
	DomainSuffix *OptionalString `json:",omitempty"`

	// Optional override of HTTP API that acts as ACME DNS-01 Challenge broker
	RegistrationEndpoint *OptionalString `json:",omitempty"`

	// Optional Authorization token, used with private/test instances of p2p-forge
	RegistrationToken *OptionalString `json:",omitempty"`

	// Optional override of CA ACME API used by p2p-forge system
	CAEndpoint *OptionalString `json:",omitempty"`
}

const (
	DefaultAutoTLSEnabled       = false // experimental, opt-in for now (https://github.com/ipfs/kubo/pull/10521)
	DefaultDomainSuffix         = p2pforge.DefaultForgeDomain
	DefaultRegistrationEndpoint = p2pforge.DefaultForgeEndpoint
	DefaultCAEndpoint           = p2pforge.DefaultCAEndpoint
)
