package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestInternalCheckFlagsDefaultEnabled locks the contract that the CGNAT and
// dead-listener diagnostics are enabled when the operator has not set them.
func TestInternalCheckFlagsDefaultEnabled(t *testing.T) {
	var in Internal // zero value: nothing set
	if !in.CGNATCheck.WithDefault(DefaultCGNATCheck) {
		t.Error("CGNATCheck should default to enabled")
	}
	if !in.DeadListenerCheck.WithDefault(DefaultDeadListenerCheck) {
		t.Error("DeadListenerCheck should default to enabled")
	}
}

// TestInternalCheckFlagsJSON verifies the flags are omitted from JSON when
// unset (Default) and round-trip to disabled when set to false.
func TestInternalCheckFlagsJSON(t *testing.T) {
	out, err := json.Marshal(Internal{})
	if err != nil {
		t.Fatal(err)
	}
	if s := string(out); strings.Contains(s, "CGNATCheck") || strings.Contains(s, "DeadListenerCheck") {
		t.Errorf("unset check flags must be omitted from JSON, got: %s", s)
	}

	var in Internal
	if err := json.Unmarshal([]byte(`{"CGNATCheck":false,"DeadListenerCheck":false}`), &in); err != nil {
		t.Fatal(err)
	}
	if in.CGNATCheck.WithDefault(DefaultCGNATCheck) {
		t.Error("CGNATCheck=false should disable the check")
	}
	if in.DeadListenerCheck.WithDefault(DefaultDeadListenerCheck) {
		t.Error("DeadListenerCheck=false should disable the check")
	}
}
