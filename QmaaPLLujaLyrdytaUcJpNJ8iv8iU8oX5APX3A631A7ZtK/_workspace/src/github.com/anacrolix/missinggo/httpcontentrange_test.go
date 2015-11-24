package missinggo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHTTPContentRange(t *testing.T) {
	for _, _case := range []struct {
		h  string
		cr *HTTPBytesContentRange
	}{
		{"", nil},
		{"1-2/*", nil},
		{"bytes=1-2/3", &HTTPBytesContentRange{1, 2, 3}},
		{"bytes=12-34/*", &HTTPBytesContentRange{12, 34, -1}},
		{" bytes=12-34/*", &HTTPBytesContentRange{12, 34, -1}},
		{"  bytes 12-34/56", &HTTPBytesContentRange{12, 34, 56}},
	} {
		ret, ok := ParseHTTPBytesContentRange(_case.h)
		assert.Equal(t, _case.cr != nil, ok)
		if _case.cr != nil {
			assert.Equal(t, *_case.cr, ret)
		}
	}
}
