package pebbleds

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble/v2"
	pebbleds "github.com/ipfs/go-ds-pebble"
	"github.com/ipfs/kubo/misc/fsutil"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
)

// Plugins is exported list of plugins that will be loaded.
var Plugins = []plugin.Plugin{
	&pebbledsPlugin{},
}

// Defaults for pebble's experimental value separation, applied when
// "valueSeparationEnabled" is true and the corresponding option is unset.
// Pebble has no defaults of its own here (a nil policy means disabled), so
// these are Kubo's choices: separate everything above the size of a typical
// small DAG node, and follow the reference values used in pebble's own
// ValueSeparationPolicy documentation for the rest.
const (
	DefaultValueSeparationMinimumSize           = 1024
	DefaultValueSeparationMaxBlobReferenceDepth = 10
	DefaultValueSeparationRewriteMinimumAge     = 15 * time.Minute
	DefaultValueSeparationTargetGarbageRatio    = 0.20
)

type pebbledsPlugin struct{}

var _ plugin.PluginDatastore = (*pebbledsPlugin)(nil)

func (*pebbledsPlugin) Name() string {
	return "ds-pebble"
}

func (*pebbledsPlugin) Version() string {
	return "0.1.0"
}

func (*pebbledsPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*pebbledsPlugin) DatastoreTypeName() string {
	return "pebbleds"
}

type datastoreConfig struct {
	path      string
	cacheSize int64

	// Documentation of these values: https://pkg.go.dev/github.com/cockroachdb/pebble@v1.1.2#Options
	pebbleOpts *pebble.Options
}

// PebbleDatastoreConfig returns a configuration stub for a pebble datastore
// from the given parameters.
func (*pebbledsPlugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(params map[string]any) (fsrepo.DatastoreConfig, error) {
		var c datastoreConfig
		var ok bool

		c.path, ok = params["path"].(string)
		if !ok {
			return nil, fmt.Errorf("'path' field is missing or not string")
		}

		cacheSize, err := getConfigInt("cacheSize", params)
		if err != nil {
			return nil, err
		}
		c.cacheSize = int64(cacheSize)

		bytesPerSync, err := getConfigInt("bytesPerSync", params)
		if err != nil {
			return nil, err
		}
		disableWAL, err := getConfigBool("disableWAL", params)
		if err != nil {
			return nil, err
		}
		fmv, err := getConfigInt("formatMajorVersion", params)
		if err != nil {
			return nil, err
		}
		formatMajorVersion := pebble.FormatMajorVersion(fmv)
		l0CompactionThreshold, err := getConfigInt("l0CompactionThreshold", params)
		if err != nil {
			return nil, err
		}
		l0StopWritesThreshold, err := getConfigInt("l0StopWritesThreshold", params)
		if err != nil {
			return nil, err
		}
		lBaseMaxBytes, err := getConfigInt("lBaseMaxBytes", params)
		if err != nil {
			return nil, err
		}
		maxConcurrentCompactions, err := getConfigInt("maxConcurrentCompactions", params)
		if err != nil {
			return nil, err
		}
		memTableSize, err := getConfigInt("memTableSize", params)
		if err != nil {
			return nil, err
		}
		memTableStopWritesThreshold, err := getConfigInt("memTableStopWritesThreshold", params)
		if err != nil {
			return nil, err
		}
		walBytesPerSync, err := getConfigInt("walBytesPerSync", params)
		if err != nil {
			return nil, err
		}
		walMinSyncSec, err := getConfigInt("walMinSyncIntervalSeconds", params)
		if err != nil {
			return nil, err
		}
		valueSepEnabled, err := getConfigBool("valueSeparationEnabled", params)
		if err != nil {
			return nil, err
		}
		valueSepMinSize, err := getConfigInt("valueSeparationMinimumSize", params)
		if err != nil {
			return nil, err
		}
		valueSepMaxRefDepth, err := getConfigInt("valueSeparationMaxBlobReferenceDepth", params)
		if err != nil {
			return nil, err
		}
		valueSepRewriteAgeSec, err := getConfigInt("valueSeparationRewriteMinimumAgeSeconds", params)
		if err != nil {
			return nil, err
		}
		valueSepGarbageRatio, err := getConfigFloat("valueSeparationTargetGarbageRatio", params)
		if err != nil {
			return nil, err
		}
		if !valueSepEnabled && (valueSepMinSize != 0 || valueSepMaxRefDepth != 0 || valueSepRewriteAgeSec != 0 || valueSepGarbageRatio != 0) {
			return nil, fmt.Errorf("valueSeparation* options require \"valueSeparationEnabled\": true")
		}

		if formatMajorVersion == 0 {
			// Pebble DB format not configured. Automatically ratchet the
			// database to the latest format. This may prevent downgrade.
			formatMajorVersion = pebble.FormatNewest
		} else if formatMajorVersion < pebble.FormatNewest {
			// Pebble DB format is configured, but is not the latest.
			fmt.Println("⚠️ A newer pebble db format is available.")
			fmt.Println("  To upgrade, set the following in the pebble datastore config:")
			fmt.Println("    \"formatMajorVersion\":", int(pebble.FormatNewest))
		}

		if bytesPerSync != 0 || disableWAL || formatMajorVersion != 0 || l0CompactionThreshold != 0 || l0StopWritesThreshold != 0 || lBaseMaxBytes != 0 || maxConcurrentCompactions != 0 || memTableSize != 0 || memTableStopWritesThreshold != 0 || walBytesPerSync != 0 || walMinSyncSec != 0 {
			c.pebbleOpts = &pebble.Options{
				BytesPerSync:                bytesPerSync,
				DisableWAL:                  disableWAL,
				FormatMajorVersion:          formatMajorVersion,
				L0CompactionThreshold:       l0CompactionThreshold,
				L0StopWritesThreshold:       l0StopWritesThreshold,
				LBaseMaxBytes:               int64(lBaseMaxBytes),
				MemTableSize:                uint64(memTableSize),
				MemTableStopWritesThreshold: memTableStopWritesThreshold,
				WALBytesPerSync:             walBytesPerSync,
			}
			if maxConcurrentCompactions != 0 {
				c.pebbleOpts.CompactionConcurrencyRange = func() (int, int) { return 1, maxConcurrentCompactions }
			}
			if walMinSyncSec != 0 {
				c.pebbleOpts.WALMinSyncInterval = func() time.Duration { return time.Duration(walMinSyncSec) * time.Second }
			}
		}

		if valueSepEnabled {
			// Blob files are only readable at FormatValueSeparation or
			// newer. Refuse to silently ratchet a repo that pinned an
			// older format: raising formatMajorVersion is irreversible
			// and must stay an explicit user decision.
			if formatMajorVersion < pebble.FormatValueSeparation {
				return nil, fmt.Errorf(
					"valueSeparationEnabled requires \"formatMajorVersion\" >= %d (currently %d); raising it is IRREVERSIBLE, see docs/datastores.md#pebbleds",
					int(pebble.FormatValueSeparation), int(formatMajorVersion))
			}
			if valueSepMinSize < 0 || valueSepMaxRefDepth < 0 || valueSepRewriteAgeSec < 0 {
				return nil, fmt.Errorf("valueSeparation* options must not be negative")
			}
			if valueSepGarbageRatio < 0 || valueSepGarbageRatio > 1 {
				return nil, fmt.Errorf("\"valueSeparationTargetGarbageRatio\" must be within [0.0, 1.0]")
			}
			if valueSepMinSize == 0 {
				valueSepMinSize = DefaultValueSeparationMinimumSize
			}
			if valueSepMaxRefDepth == 0 {
				valueSepMaxRefDepth = DefaultValueSeparationMaxBlobReferenceDepth
			}
			rewriteAge := DefaultValueSeparationRewriteMinimumAge
			if valueSepRewriteAgeSec != 0 {
				rewriteAge = time.Duration(valueSepRewriteAgeSec) * time.Second
			}
			if valueSepGarbageRatio == 0 {
				valueSepGarbageRatio = DefaultValueSeparationTargetGarbageRatio
			}
			if c.pebbleOpts == nil {
				c.pebbleOpts = &pebble.Options{FormatMajorVersion: formatMajorVersion}
			}
			policy := pebble.ValueSeparationPolicy{
				Enabled:               true,
				MinimumSize:           valueSepMinSize,
				MaxBlobReferenceDepth: valueSepMaxRefDepth,
				RewriteMinimumAge:     rewriteAge,
				TargetGarbageRatio:    valueSepGarbageRatio,
			}
			// The policy is ignored unless Experimental.EnableColumnarBlocks
			// is true; go-ds-pebble calls EnsureDefaults, which sets it.
			c.pebbleOpts.Experimental.ValueSeparationPolicy = func() pebble.ValueSeparationPolicy { return policy }
		}

		return &c, nil
	}
}

func getConfigBool(name string, params map[string]any) (bool, error) {
	val, ok := params[name]
	if ok {
		bval, ok := val.(bool)
		if !ok {
			return false, fmt.Errorf("%q field was not a bool", name)
		}
		return bval, nil
	}
	return false, nil
}

func getConfigInt(name string, params map[string]any) (int, error) {
	val, ok := params[name]
	if ok {
		// TODO: see why val may be an int or a float64.
		ival, ok := val.(int)
		if !ok {
			fval, ok := val.(float64)
			if !ok {
				return 0, fmt.Errorf("%q field was not an integer or a float64", name)
			}
			return int(fval), nil
		}
		return ival, nil
	}
	return 0, nil
}

func getConfigFloat(name string, params map[string]any) (float64, error) {
	val, ok := params[name]
	if ok {
		fval, ok := val.(float64)
		if !ok {
			ival, ok := val.(int)
			if !ok {
				return 0, fmt.Errorf("%q field was not a float64 or an integer", name)
			}
			return float64(ival), nil
		}
		return fval, nil
	}
	return 0, nil
}

func (c *datastoreConfig) DiskSpec() fsrepo.DiskSpec {
	return map[string]any{
		"type": "pebbleds",
		"path": c.path,
	}
}

func (c *datastoreConfig) Create(path string) (repo.Datastore, error) {
	p := c.path
	if !filepath.IsAbs(p) {
		p = filepath.Join(path, p)
	}

	if err := fsutil.DirWritable(p); err != nil {
		return nil, err
	}

	return pebbleds.NewDatastore(p, pebbleds.WithCacheSize(c.cacheSize), pebbleds.WithPebbleOpts(c.pebbleOpts))
}
