package kubo

import (
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/require"
)

func TestValidateDaemonConfig(t *testing.T) {
	tests := []struct {
		name           string
		cfg            func() *config.Config
		routingOption  string
		privateNetwork bool
		errContains    string
	}{
		{
			name:          "default config is valid",
			cfg:           func() *config.Config { return &config.Config{} },
			routingOption: routingOptionAutoKwd,
		},
		{
			name:           "private network rejects auto routing",
			cfg:            func() *config.Config { return &config.Config{} },
			routingOption:  routingOptionAutoKwd,
			privateNetwork: true,
			errContains:    "private network does not work with Routing.Type=auto",
		},
		{
			name: "deprecated Provider config is rejected",
			cfg: func() *config.Config {
				return &config.Config{
					Provider: config.Provider{
						Enabled: config.True,
					},
				}
			},
			routingOption: routingOptionAutoKwd,
			errContains:   "Manually migrate 'Provider' fields",
		},
		{
			name: "deprecated Reprovider config is rejected",
			cfg: func() *config.Config {
				return &config.Config{
					Reprovider: config.Reprovider{
						Strategy: config.NewOptionalString("all"),
					},
				}
			},
			routingOption: routingOptionAutoKwd,
			errContains:   "Manually migrate 'Reprovider' fields",
		},
		{
			name: "flat provide strategy is rejected",
			cfg: func() *config.Config {
				return &config.Config{
					Provide: config.Provide{
						Strategy: config.NewOptionalString("flat"),
					},
				}
			},
			routingOption: routingOptionAutoKwd,
			errContains:   "Provide.Strategy='flat' is no longer supported",
		},
		{
			name: "strategic providing is rejected",
			cfg: func() *config.Config {
				return &config.Config{
					Experimental: config.Experiments{
						StrategicProviding: true,
					},
				}
			},
			routingOption: routingOptionAutoKwd,
			errContains:   "Experimental.StrategicProviding was removed",
		},
		{
			name: "sweep provider rejects zero max workers",
			cfg: func() *config.Config {
				return &config.Config{
					Provide: config.Provide{
						DHT: config.ProvideDHT{
							MaxWorkers: config.NewOptionalInteger(0),
						},
					},
				}
			},
			routingOption: routingOptionAutoKwd,
			errContains:   "Provide.DHT.MaxWorkers cannot be 0",
		},
		{
			name:          "delegated routing rejects enabled providing",
			cfg:           func() *config.Config { return &config.Config{} },
			routingOption: routingOptionDelegatedKwd,
			errContains:   "Routing.Type=delegated does not support content providing",
		},
		{
			name: "delegated routing allows disabled providing",
			cfg: func() *config.Config {
				return &config.Config{
					Provide: config.Provide{
						Enabled: config.False,
					},
				}
			},
			routingOption: routingOptionDelegatedKwd,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDaemonConfig(test.cfg(), test.routingOption, test.privateNetwork)
			if test.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.errContains)
			}
		})
	}
}

func TestValidateDaemonEnvironmentRejectsRemovedReuseport(t *testing.T) {
	t.Setenv("IPFS_REUSEPORT", "true")

	err := validateDaemonEnvironment()

	require.ErrorContains(t, err, "IPFS_REUSEPORT was removed")
}
