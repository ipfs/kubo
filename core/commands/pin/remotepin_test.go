package pin

import (
	"testing"
)

func TestNormalizeEndpoint(t *testing.T) {
	cases := []struct {
		in  string
		err string
		out string
	}{
		{
			in:  "https://1.example.com",
			err: "",
			out: "https://1.example.com",
		},
		{
			in:  "https://2.example.com/",
			err: "",
			out: "https://2.example.com",
		},
		{
			in:  "https://3.example.com/pins/",
			err: "",
			out: "https://3.example.com",
		},
		{
			in:  "https://4.example.com/pins",
			err: "",
			out: "https://4.example.com",
		},
		{
			in:  "http://192.168.0.5:45000/pins",
			err: "",
			out: "http://192.168.0.5:45000",
		},
		{
			in:  "foo://4.example.com/pins",
			err: "service endpoint must be a valid HTTP URL",
			out: "",
		},
	}

	for _, tc := range cases {
		out, err := normalizeEndpoint(tc.in)
		if out != tc.out {
			t.Errorf("unexpected endpoint for %q: expected %q; got %q", tc.in, tc.out, out)
			continue
		}
		if err != nil && tc.err != err.Error() {
			t.Errorf("unexpected error for %q: expected %q; got %q", tc.in, tc.err, err)
			continue
		}
	}

}
