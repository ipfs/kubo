package config

import (
	"strings"
	"testing"
)

func TestAutoUpdateValues(t *testing.T) {
	var tval struct {
		AutoUpdate AutoUpdateSetting
	}
	tests := []struct {
		input string
		val   AutoUpdateSetting
		err   error
	}{
		{`{"hello":123}`, AutoUpdateNever, nil}, // zero value
		{`{"AutoUpdate": "never"}`, AutoUpdateNever, nil},
		{`{"AutoUpdate": "patch"}`, AutoUpdatePatch, nil},
		{`{"AutoUpdate": "minor"}`, AutoUpdateMinor, nil},
		{`{"AutoUpdate": "major"}`, AutoUpdateMajor, nil},
		{`{"AutoUpdate": "blarg"}`, AutoUpdateMinor, ErrUnknownAutoUpdateSetting},
	}

	for i, tc := range tests {
		err := Decode(strings.NewReader(tc.input), &tval)
		if err != tc.err {
			t.Fatalf("%d failed - got err %q wanted %v", i, err, tc.err)
		}

		if tval.AutoUpdate != tc.val {
			t.Fatalf("%d failed - got val %q where we wanted %q", i, tval.AutoUpdate, tc.val)
		}
	}

}
