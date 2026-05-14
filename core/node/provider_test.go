package node

import (
	"context"
	"math"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDatastore returns a fresh in-memory datastore for unique-count
// persistence tests. Tests are single-goroutine so no sync wrapper is
// needed.
func newTestDatastore() datastore.Datastore {
	return datastore.NewMapDatastore()
}

func TestReadLastUniqueCount_emptyReturnsZero(t *testing.T) {
	ds := newTestDatastore()

	// A fresh datastore has no persisted count. The reader treats this
	// as "no previous cycle data available" and returns 0, which the
	// caller falls back to DefaultBloomInitialCapacity for.
	got := readLastUniqueCount(ds)
	assert.Equal(t, uint64(0), got)
}

func TestPersistAndReadUniqueCount_roundTrip(t *testing.T) {
	tests := []struct {
		name  string
		count uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"small", 1_000},
		{"million", 1_000_000},
		{"billion", 1_000_000_000},
		{"max uint64", math.MaxUint64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := newTestDatastore()
			persistUniqueCount(ds, tt.count)
			got := readLastUniqueCount(ds)
			assert.Equal(t, tt.count, got)
		})
	}
}

func TestPersistUniqueCount_overwriteReplacesPreviousValue(t *testing.T) {
	ds := newTestDatastore()

	// Each reprovide cycle persists a new count, overwriting the
	// previous one. The reader must return the most recent value.
	persistUniqueCount(ds, 1_000)
	persistUniqueCount(ds, 2_000_000)
	persistUniqueCount(ds, 42)

	got := readLastUniqueCount(ds)
	assert.Equal(t, uint64(42), got)
}

func TestReadLastUniqueCount_corruptLengthReturnsZero(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"empty bytes", []byte{}},
		{"too short (4 bytes)", []byte{0x01, 0x02, 0x03, 0x04}},
		{"too long (16 bytes)", make([]byte, 16)},
		{"single byte", []byte{0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := newTestDatastore()
			// Write malformed bytes directly under the persistence key
			// to simulate a corrupt or truncated entry.
			err := ds.Put(context.Background(), datastore.NewKey(reprovideLastUniqueCountKey), tt.raw)
			require.NoError(t, err)

			// The reader rejects anything that is not exactly 8 bytes
			// and falls back to 0 instead of panicking on a short read.
			got := readLastUniqueCount(ds)
			assert.Equal(t, uint64(0), got)
		})
	}
}
