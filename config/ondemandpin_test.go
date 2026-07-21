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

	t.Run("zero replication min is rejected", func(t *testing.T) {
		cfg := &OnDemandPinning{ReplicationTargetMin: *NewOptionalInteger(0)}
		err := ValidateOnDemandPinningConfig(cfg)
		assert.ErrorContains(t, err, "ReplicationTargetMin")
	})

	t.Run("max below min is rejected", func(t *testing.T) {
		cfg := &OnDemandPinning{
			ReplicationTargetMin: *NewOptionalInteger(5),
			ReplicationTargetMax: *NewOptionalInteger(4),
		}
		err := ValidateOnDemandPinningConfig(cfg)
		assert.ErrorContains(t, err, "ReplicationTargetMax")
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

func TestValidateOnDemandPinningRouting(t *testing.T) {
	assert.NoError(t, ValidateOnDemandPinningRouting("auto"))
	assert.NoError(t, ValidateOnDemandPinningRouting("dht"))
	assert.NoError(t, ValidateOnDemandPinningRouting("delegated"))
	assert.ErrorContains(t, ValidateOnDemandPinningRouting("none"), "none")
}
