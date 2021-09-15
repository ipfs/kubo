package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestPins(t *testing.T) {
	cases := []struct {
		name     string
		baseArgs []string
		pinArgs  []string
		lsArgs   []string
	}{
		{
			name: "happy",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := harness.NewForTest(t)
			h.Init()

			f := h.TempFile()
			hashFileName := f.Name()
			defer f.Close()

			// test_pins pinargs='' lsargs='' base=''
			// create some hashes

			// add some data and write the cids out to a file in the format expected by the pin cmd
			strs := []string{"a", "b", "c", "d", "e", "f", "g"}
			cids := map[string]string{}
			for _, s := range strs {
				cidStr := h.IPFSAddStr(s, StrConcat(c.baseArgs, "--pin=false")...)

				// validate the cid and store mapping from data -> cid
				_, err := cid.Decode(cidStr)
				assert.NoError(t, err)
				cids[s] = cidStr

				f.WriteString(cidStr + "\n")
			}

			f.Close()

			// pin the hashes from the file
			hashFile := MustOpen(hashFileName)
			res := h.Runner.MustRun(harness.RunRequest{
				Path:    h.IPFSBin,
				Args:    StrConcat(c.baseArgs, "pin", "add"),
				CmdOpts: []harness.CmdOpt{h.Runner.RunWithStdin(hashFile)},
			})

			t.Run("check output of pin command", func(t *testing.T) {
				lines := strings.Split(res.Stdout.String(), "\n")
				for i, s := range strs {
					assert.Equal(t,
						fmt.Sprintf("pinned %s recursively", cids[s]),
						lines[i],
					)
				}
			})
			t.Run("pin verify should succeed", func(t *testing.T) {
				h.MustRunIPFS("pin", "verify")
			})
			t.Run("'pin verify --verbose' should include all the cids", func(t *testing.T) {
				verboseVerifyOut := h.MustRunIPFS(StrConcat(c.baseArgs, "pin", "verify", "--verbose")...).Stdout.String()
				for _, cid := range cids {
					assert.Contains(t, verboseVerifyOut, fmt.Sprintf("%s ok", cid))
				}

			})
			t.Run("ls output should contain the cids", func(t *testing.T) {
				lsOut := h.MustRunIPFS(StrConcat("pin", "ls", c.lsArgs, c.baseArgs)...).Stdout.String()
				for _, cid := range cids {
					assert.Contains(t, lsOut, cid)
				}
			})
			t.Run("check 'pin ls hash' output", func(t *testing.T) {
				lsHashOut := h.MustRunIPFS(StrConcat("pin", "ls", c.lsArgs, c.baseArgs, cids["b"])...)
				lsHashOutStr := lsHashOut.Stdout.String()
				assert.Equal(t, fmt.Sprintf("%s recursive\n", cids["b"]), lsHashOutStr)
			})

			// unpin the hashes
			h.MustRunIPFS("pin", "rm")
		})
	}

}
