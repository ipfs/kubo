package corerepo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckDiskFull(t *testing.T) {
	repoDir := t.TempDir()

	full, err := checkDiskFull(repoDir, 99.9)
	require.NoError(t, err)
	require.False(t, full)

	full, err = checkDiskFull(repoDir, 0.01)
	require.NoError(t, err)
	require.True(t, full)

	_, err = checkDiskFull("/no/such/directory", 90)
	require.Error(t, err)
}
