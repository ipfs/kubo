package config

import (
	"fmt"
)

// AutoNATServiceMode configures the ipfs node's AutoNAT service.
type AutoNATServiceMode int

const (
	// AutoNATServiceUnset indicates that the user has not set the
	// AutoNATService mode.
	//
	// When unset, nodes configured to be public DHT nodes will _also_
	// perform limited AutoNAT dialbacks.
	AutoNATServiceUnset AutoNATServiceMode = iota
	// AutoNATServiceEnabled indicates that the user has enabled the
	// AutoNATService.
	AutoNATServiceEnabled
	// AutoNATServiceDisabled indicates that the user has disabled the
	// AutoNATService.
	AutoNATServiceDisabled
	// AutoNATServiceEnabledV1Only forces use of V1 and disables V2
	// (used for testing)
	AutoNATServiceEnabledV1Only
)

func (m *AutoNATServiceMode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "":
		*m = AutoNATServiceUnset
	case "enabled":
		*m = AutoNATServiceEnabled
	case "disabled":
		*m = AutoNATServiceDisabled
	case "legacy-v1":
		*m = AutoNATServiceEnabledV1Only
	default:
		return fmt.Errorf("unknown autonat mode: %s", string(text))
	}
	return nil
}

func (m AutoNATServiceMode) MarshalText() ([]byte, error) {
	switch m {
	case AutoNATServiceUnset:
		return nil, nil
	case AutoNATServiceEnabled:
		return []byte("enabled"), nil
	case AutoNATServiceDisabled:
		return []byte("disabled"), nil
	case AutoNATServiceEnabledV1Only:
		return []byte("legacy-v1"), nil
	default:
		return nil, fmt.Errorf("unknown autonat mode: %d", m)
	}
}

// AutoNATConfig configures the node's AutoNAT subsystem.
type AutoNATConfig struct {
	// ServiceMode configures the node's AutoNAT service mode.
	ServiceMode AutoNATServiceMode `json:",omitempty"`

	// Throttle configures AutoNAT dialback throttling.
	//
	// If unset, the conservative libp2p defaults will be unset. To help the
	// network, please consider setting this and increasing the limits.
	//
	// By default, the limits will be a total of 30 dialbacks, with a
	// per-peer max of 3 peer, resetting every minute.
	Throttle *AutoNATThrottleConfig `json:",omitempty"`
}

// AutoNATThrottleConfig configures the throttle limites.
type AutoNATThrottleConfig struct {
	// GlobalLimit and PeerLimit sets the global and per-peer dialback
	// limits. The AutoNAT service will only perform the specified number of
	// dialbacks per interval.
	//
	// Setting either to 0 will disable the appropriate limit.
	GlobalLimit, PeerLimit int

	// Interval specifies how frequently this node should reset the
	// global/peer dialback limits.
	//
	// When unset, this defaults to 1 minute.
	Interval OptionalDuration `json:",omitempty"`
}
