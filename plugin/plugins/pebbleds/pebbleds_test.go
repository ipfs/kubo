package pebbleds

import (
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
)

func parseConfig(t *testing.T, params map[string]any) (*datastoreConfig, error) {
	t.Helper()
	p := &pebbledsPlugin{}
	cfg, err := p.DatastoreConfigParser()(params)
	if err != nil {
		return nil, err
	}
	return cfg.(*datastoreConfig), nil
}

func TestValueSeparationDisabledByDefault(t *testing.T) {
	c, err := parseConfig(t, map[string]any{
		"type": "pebbleds",
		"path": "pebbleds",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.pebbleOpts != nil && c.pebbleOpts.Experimental.ValueSeparationPolicy != nil {
		if c.pebbleOpts.Experimental.ValueSeparationPolicy().Enabled {
			t.Fatal("value separation must be disabled unless requested")
		}
	}
}

func TestValueSeparationEnabledDefaults(t *testing.T) {
	c, err := parseConfig(t, map[string]any{
		"type":                   "pebbleds",
		"path":                   "pebbleds",
		"valueSeparationEnabled": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.pebbleOpts == nil || c.pebbleOpts.Experimental.ValueSeparationPolicy == nil {
		t.Fatal("expected a value separation policy to be configured")
	}
	got := c.pebbleOpts.Experimental.ValueSeparationPolicy()
	want := pebble.ValueSeparationPolicy{
		Enabled:               true,
		MinimumSize:           DefaultValueSeparationMinimumSize,
		MaxBlobReferenceDepth: DefaultValueSeparationMaxBlobReferenceDepth,
		RewriteMinimumAge:     DefaultValueSeparationRewriteMinimumAge,
		TargetGarbageRatio:    DefaultValueSeparationTargetGarbageRatio,
	}
	if got != want {
		t.Fatalf("policy = %+v, want %+v", got, want)
	}
	if c.pebbleOpts.FormatMajorVersion < pebble.FormatValueSeparation {
		t.Fatalf("format %d does not support value separation", c.pebbleOpts.FormatMajorVersion)
	}
}

func TestValueSeparationEnabledCustomValues(t *testing.T) {
	// JSON numbers arrive as float64, mimic that.
	c, err := parseConfig(t, map[string]any{
		"type":                                 "pebbleds",
		"path":                                 "pebbleds",
		"valueSeparationEnabled":               true,
		"valueSeparationMinimumSize":           float64(4096),
		"valueSeparationMaxBlobReferenceDepth": float64(5),
		"valueSeparationRewriteMinimumAgeSeconds": float64(60),
		"valueSeparationTargetGarbageRatio":       0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := c.pebbleOpts.Experimental.ValueSeparationPolicy()
	want := pebble.ValueSeparationPolicy{
		Enabled:               true,
		MinimumSize:           4096,
		MaxBlobReferenceDepth: 5,
		RewriteMinimumAge:     60 * time.Second,
		TargetGarbageRatio:    0.5,
	}
	if got != want {
		t.Fatalf("policy = %+v, want %+v", got, want)
	}
}

func TestValueSeparationRequiresSupportedFormat(t *testing.T) {
	_, err := parseConfig(t, map[string]any{
		"type":                   "pebbleds",
		"path":                   "pebbleds",
		"valueSeparationEnabled": true,
		"formatMajorVersion":     float64(pebble.FormatValueSeparation - 1),
	})
	if err == nil {
		t.Fatal("expected error for formatMajorVersion below FormatValueSeparation")
	}
	if !strings.Contains(err.Error(), "formatMajorVersion") {
		t.Fatalf("error should mention formatMajorVersion, got: %v", err)
	}
}

func TestValueSeparationOptionsRequireEnabled(t *testing.T) {
	_, err := parseConfig(t, map[string]any{
		"type":                       "pebbleds",
		"path":                       "pebbleds",
		"valueSeparationMinimumSize": float64(4096),
	})
	if err == nil {
		t.Fatal("expected error for valueSeparation* option without valueSeparationEnabled")
	}
}

func TestValueSeparationGarbageRatioValidation(t *testing.T) {
	_, err := parseConfig(t, map[string]any{
		"type":                              "pebbleds",
		"path":                              "pebbleds",
		"valueSeparationEnabled":            true,
		"valueSeparationTargetGarbageRatio": 1.5,
	})
	if err == nil {
		t.Fatal("expected error for garbage ratio > 1.0")
	}
}
