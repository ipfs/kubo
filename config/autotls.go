package config

import (
	"time"

	p2pforge "github.com/ipshipyard/p2p-forge/client"
)

// AutoTLS includes optional configuration of p2p-forge client of service
// for obtaining a domain and TLS certificate to improve connectivity for web
// browser clients. More: https://github.com/ipshipyard/p2p-forge#readme
type AutoTLS struct {
	// Enables the p2p-forge feature and all related features.
	Enabled Flag `json:",omitempty"`

	// Optional, controls if Kubo should add /tls/sni/.../ws listener to every /tcp port if no explicit /ws is defined in Addresses.Swarm
	AutoWSS Flag `json:",omitempty"`

	// Optional, controls whether to skip network DNS lookups for p2p-forge domains.
	// Applies to resolution via DNS.Resolvers, including /dns* multiaddrs in go-libp2p.
	// When enabled (default), A/AAAA queries for *.libp2p.direct are resolved
	// locally by parsing the IP directly from the hostname, avoiding network I/O.
	// Set to false to always use network DNS (useful for debugging).
	SkipDNSLookup Flag `json:",omitempty"`

	// Optional override of the parent domain that will be used
	DomainSuffix *OptionalString `json:",omitempty"`

	// Optional override of HTTP API that acts as ACME DNS-01 Challenge broker
	RegistrationEndpoint *OptionalString `json:",omitempty"`

	// Optional Authorization token, used with private/test instances of p2p-forge
	RegistrationToken *OptionalString `json:",omitempty"`

	// Optional registration delay used when AutoTLS.Enabled is not explicitly set to true in config
	RegistrationDelay *OptionalDuration `json:",omitempty"`

	// Optional override of CA ACME API used by p2p-forge system
	CAEndpoint *OptionalString `json:",omitempty"`

	// Optional, controls if features like AutoWSS should generate shorter /dnsX instead of /ipX/../sni/..
	ShortAddrs Flag `json:",omitempty"`

	// SelfSignedForTests is a test-only escape hatch. When true, the daemon
	// skips the AutoTLS / p2p-forge / ACME pipeline entirely and provides
	// the WebSocket transport with an in-memory self-signed TLS config.
	// Test clients pair this with tls.Config{InsecureSkipVerify: true} to
	// drive the /tls/ws and /tls/http paths without real ACME issuance.
	//
	// Never set this in a production config. The cert is regenerated on
	// every daemon start and is not trusted by any browser or CA.
	SelfSignedForTests Flag `json:",omitempty"`

	// TrustedCARootsPEM is an optional PEM-encoded bundle of CA
	// certificates that the ACME client trusts in addition to the system
	// trust store. Set this when AutoTLS.CAEndpoint points at a CA whose
	// root is not in the system store: private or self-hosted ACME
	// deployments, and the in-process Pebble used by the AutoTLS E2E test
	// in test/autotls/.
	TrustedCARootsPEM *OptionalString `json:",omitempty"`

	// AllowPrivateForgeAddrs lifts the p2p-forge client's requirement that
	// the libp2p host report a publicly reachable address before requesting
	// a certificate. Set this for private/intranet libp2p deployments
	// (where reachability is asymmetric or implicit) and for the AutoTLS
	// E2E test in test/autotls/, which runs entirely on loopback.
	//
	// Leave this off in normal public deployments; the default behavior
	// avoids wasting ACME issuance on a node that no one can reach.
	AllowPrivateForgeAddrs Flag `json:",omitempty"`
}

const (
	DefaultAutoTLSEnabled            = true // with DefaultAutoTLSRegistrationDelay, unless explicitly enabled  in config
	DefaultDomainSuffix              = p2pforge.DefaultForgeDomain
	DefaultRegistrationEndpoint      = p2pforge.DefaultForgeEndpoint
	DefaultCAEndpoint                = p2pforge.DefaultCAEndpoint
	DefaultAutoWSS                   = true // requires AutoTLS.Enabled
	DefaultAutoTLSShortAddrs         = true // requires AutoTLS.Enabled
	DefaultAutoTLSSkipDNSLookup      = true // skip network DNS for p2p-forge domains
	DefaultAutoTLSRegistrationDelay  = 1 * time.Hour
	DefaultAutoTLSSelfSignedForTests = false
)
