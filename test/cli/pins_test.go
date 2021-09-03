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
		cidStr := h.IPFSAdd(strings.NewReader(s), "--pin=false")

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
		Args:    []string{"pin", "add"},
		CmdOpts: []harness.CmdOpt{h.Runner.RunWithStdin(hashFile)},
	})

	// validate the output of the pin command, we should see one line for each datum
	lines := strings.Split(res.Stdout.String(), "\n")
	for i, s := range strs {
		assert.Equal(t,
			fmt.Sprintf("pinned %s recursively", cids[s]),
			lines[i],
		)
	}

	h.MustRunIPFS("pin", "verify")

	// pin verify --verbose should include all the cids
	verboseVerifyOut := h.MustRunIPFS("pin", "verify", "--verbose").Stdout.String()
	for _, cid := range cids {
		assert.Contains(t, verboseVerifyOut, fmt.Sprintf("%s ok", cid))
	}

}
