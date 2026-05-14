package commands

import (
	"testing"
)

func TestCommandTree(t *testing.T) {
	printErrors := func(errs map[string][]error) {
		if errs == nil {
			return
		}
		t.Error("In Root command tree:")
		for cmd, err := range errs {
			t.Errorf("  In X command %s:", cmd)
			for _, e := range err {
				t.Errorf("    %s", e)
			}
		}
	}
	printErrors(Root.DebugValidate())
}
