package cmdutils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathOrCidPath(t *testing.T) {
	t.Run("valid path is returned as-is", func(t *testing.T) {
		validPath := "/ipfs/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
		p, err := PathOrCidPath(validPath)
		require.NoError(t, err)
		assert.Equal(t, validPath, p.String())
	})

	t.Run("valid CID is converted to /ipfs/ path", func(t *testing.T) {
		cid := "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG"
		p, err := PathOrCidPath(cid)
		require.NoError(t, err)
		assert.Equal(t, "/ipfs/"+cid, p.String())
	})

	t.Run("valid ipns path is returned as-is", func(t *testing.T) {
		validPath := "/ipns/example.com"
		p, err := PathOrCidPath(validPath)
		require.NoError(t, err)
		assert.Equal(t, validPath, p.String())
	})

	t.Run("returns original error when both attempts fail", func(t *testing.T) {
		invalidInput := "invalid!@#path"
		_, err := PathOrCidPath(invalidInput)
		require.Error(t, err)

		// The error should reference the original input attempt.
		// This ensures users get meaningful error messages about their actual input.
		assert.Contains(t, err.Error(), invalidInput,
			"error should mention the original input")
		assert.Contains(t, err.Error(), "path does not have enough components",
			"error should describe the problem with the original input")
	})

	t.Run("empty string returns error about original input", func(t *testing.T) {
		_, err := PathOrCidPath("")
		require.Error(t, err)

		// Verify we're not getting an error about "/ipfs/" (the fallback)
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "/ipfs/",
			"error should be about empty input, not the fallback path")
	})

	t.Run("invalid characters return error about original input", func(t *testing.T) {
		invalidInput := "not a valid path or CID with spaces and /@#$%"
		_, err := PathOrCidPath(invalidInput)
		require.Error(t, err)

		// The error message should help debug the original input
		assert.True(t, strings.Contains(err.Error(), invalidInput) ||
			strings.Contains(err.Error(), "invalid"),
			"error should reference original problematic input")
	})

	t.Run("CID with path is converted correctly", func(t *testing.T) {
		cidWithPath := "QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG/file.txt"
		p, err := PathOrCidPath(cidWithPath)
		require.NoError(t, err)
		assert.Equal(t, "/ipfs/"+cidWithPath, p.String())
	})
}

func TestValidatePinName(t *testing.T) {
	t.Run("valid pin name is accepted", func(t *testing.T) {
		err := ValidatePinName("my-pin-name")
		assert.NoError(t, err)
	})

	t.Run("empty pin name is accepted", func(t *testing.T) {
		err := ValidatePinName("")
		assert.NoError(t, err)
	})

	t.Run("pin name at max length is accepted", func(t *testing.T) {
		maxName := strings.Repeat("a", MaxPinNameBytes)
		err := ValidatePinName(maxName)
		assert.NoError(t, err)
	})

	t.Run("pin name exceeding max length is rejected", func(t *testing.T) {
		tooLong := strings.Repeat("a", MaxPinNameBytes+1)
		err := ValidatePinName(tooLong)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max")
	})

	t.Run("pin name with unicode is counted by bytes", func(t *testing.T) {
		// Unicode character can be multiple bytes
		unicodeName := strings.Repeat("🔒", MaxPinNameBytes/4+1) // emoji is 4 bytes
		err := ValidatePinName(unicodeName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bytes")
	})
}
