package updates

import "testing"

// TestParseVersion just makes sure that we dont commit a bad version number
func TestParseVersion(t *testing.T) {
	_, err := parseVersion()
	if err != nil {
		t.Fatal(err)
	}
}
