package cmdenv

import (
	"os"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func TestShouldShowProgress(t *testing.T) {
	const flag = "progress"
	makeReq := func(opts map[string]any) *cmds.Request {
		if opts == nil {
			opts = map[string]any{}
		}
		return &cmds.Request{Options: opts}
	}

	t.Run("explicit true wins regardless of TTY", func(t *testing.T) {
		if !ShouldShowProgress(makeReq(map[string]any{flag: true}), flag) {
			t.Error("expected true for --progress=true")
		}
	})

	t.Run("explicit false wins regardless of TTY", func(t *testing.T) {
		if ShouldShowProgress(makeReq(map[string]any{flag: false}), flag) {
			t.Error("expected false for --progress=false")
		}
	})

	t.Run("unset defaults to IsTerminal(stderr)", func(t *testing.T) {
		got := ShouldShowProgress(makeReq(nil), flag)
		want := IsTerminal(os.Stderr)
		if got != want {
			t.Errorf("ShouldShowProgress(unset) = %v, want IsTerminal(os.Stderr) = %v", got, want)
		}
	})

	t.Run("non-bool value treated as unset", func(t *testing.T) {
		got := ShouldShowProgress(makeReq(map[string]any{flag: "yes"}), flag)
		want := IsTerminal(os.Stderr)
		if got != want {
			t.Errorf("ShouldShowProgress(non-bool) = %v, want IsTerminal(os.Stderr) = %v", got, want)
		}
	})
}
