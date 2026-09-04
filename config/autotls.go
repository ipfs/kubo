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

	// IPCerts lets a node that listens on TCP port 443 get a TLS certificate
	// for its own IP address straight from the certificate authority, instead
	// of registering a domain name with the p2p-forge broker. The authority
	// checks the address by connecting to it, which only works on port 443.
	//
	// The listener decides which of the two a node uses. With a
	// /ip4/../tcp/443 or /ip6/../tcp/443 listener in Addresses.Swarm, a node
	// asks only for a certificate for its own address, and if that cannot be
	// had it says so in the log and keeps trying rather than registering a
	// name instead. Without such a listener, a node uses the broker.
	IPCerts Flag `json:",omitempty"`

	// IPCertsPort overrides the TCP port the certificate authority is
	// expected to connect to when it checks an IP address. Advanced,
	// test-only: public authorities always use 443 and ignore anything else,
	// so changing this only makes sense against a local ACME server such as
	// the one in test/autotls.
	IPCertsPort *OptionalInteger `json:",omitempty"`

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

	// SelfSignedForTests is a test-only escape hatch. When true, the
	// WebSocket transport uses an in-memory self-signed TLS config instead
	// of the AutoTLS-managed certificate. It does not stop the AutoTLS /
	// p2p-forge / ACME pipeline itself; tests that want no ACME traffic at
	// all pair it with AutoTLS.Enabled=false.
	// Test clients pair this with tls.Config{InsecureSkipVerify: true} to
	// drive the /tls/ws and /tls/http paths without real ACME issuance.
	//
	// Never set this in a production config. The cert is regenerated on
	// every daemon start and is not trusted by any browser or CA.
	SelfSignedForTests Flag `json:",omitempty"`

	// TrustedCARootsPEM is an optional PEM-encoded bundle of CA
	// certificates for connections to the ACME endpoint. When set, the
	// bundle becomes the only trust anchor for those connections: the
	// system trust store is not consulted, so a bundle that does not
	// include the CA behind AutoTLS.CAEndpoint breaks issuance. Set this
	// when that endpoint is a CA whose root is not in the system store:
	// private or self-hosted ACME deployments, and the in-process Pebble
	// used by the AutoTLS E2E test in test/autotls/.
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
	DefaultAutoTLSIPCerts            = true // requires AutoTLS.Enabled and a /tcp/443 listener
	DefaultAutoTLSIPCertsPort        = p2pforge.DefaultIPCertPort
	DefaultAutoTLSShortAddrs         = true // requires AutoTLS.Enabled
	DefaultAutoTLSSkipDNSLookup      = true // skip network DNS for p2p-forge domains
	DefaultAutoTLSRegistrationDelay  = 1 * time.Hour
	DefaultAutoTLSSelfSignedForTests = false
	// DefaultAutoTLSAllowPrivateForgeAddrs stays off so public nodes do not
	// waste ACME issuance on addresses nobody can reach.
	DefaultAutoTLSAllowPrivateForgeAddrs = false
)
