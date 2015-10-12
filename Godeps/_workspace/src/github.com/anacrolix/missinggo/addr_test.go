package missinggo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitHostPort(t *testing.T) {
	assert.EqualValues(t, HostMaybePort{"a", 1, false}, SplitHostPort("a:1"))
	assert.EqualValues(t, HostMaybePort{"a", 0, true}, SplitHostPort("a"))
}

func TestHostMaybePortString(t *testing.T) {
	assert.EqualValues(t, "a:1", (HostMaybePort{"a", 1, false}).String())
	assert.EqualValues(t, "a", (HostMaybePort{"a", 0, true}).String())
}
