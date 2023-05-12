package cli

import (
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestDag(t *testing.T) {
	t.Parallel()
	t.Run("ipfs add, adds file", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		output := node.IPFSAddStr("hello world")
		output2 := node.IPFSAddStr("hello world 2")
		output3 := node.IPFSAddStr("hello world 3")
		assert.NotEqual(t, "", output)

		stat := node.RunIPFS("dag", "stat","-p",output, output2, output3)
		str := stat.Stdout.String()
		err := stat.Stderr.String()
		assert.NotEqual(t, "", str)
		assert.Nil(t, err)

	})
}
