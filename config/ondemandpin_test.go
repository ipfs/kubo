package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateOnDemandPinningConfig(t *testing.T) {
	t.Run("defaults are valid", func(t *testing.T) {
		err := ValidateOnDemandPinningConfig(&OnDemandPinning{})
		assert.NoError(t, err)
	})

	t.Run("zero replication target is rejected", func(t *testing.T) {
		cfg := &OnDemandPinning{ReplicationTarget: *NewOptionalInteger(0)}
		err := ValidateOnDemandPinningConfig(cfg)
		assert.ErrorContains(t, err, "ReplicationTarget")
	})

	t.Run("zero check interval is rejected", func(t *testing.T) {
		cfg := &OnDemandPinning{CheckInterval: *NewOptionalDuration(0)}
		err := ValidateOnDemandPinningConfig(cfg)
		assert.ErrorContains(t, err, "CheckInterval")
	})

	t.Run("negative grace period is rejected", func(t *testing.T) {
		cfg := &OnDemandPinning{UnpinGracePeriod: *NewOptionalDuration(-time.Hour)}
		err := ValidateOnDemandPinningConfig(cfg)
		assert.ErrorContains(t, err, "UnpinGracePeriod")
	})
}
