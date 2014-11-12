// +build linux darwin freebsd

package commands

import (
	"fmt"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
	rofs "github.com/jbenet/go-ipfs/fuse/readonly"
)

// amount of time to wait for mount errors
const mountTimeout = time.Second

var mountCmd = &cmds.Command{
	Description: "Mounts IPFS to the filesystem (read-only)",
	Help: `Mount ipfs at a read-only mountpoint on the OS. All ipfs objects
will be accessible under that directory. Note that the root will
not be listable, as it is virtual. Accessing known paths directly.
`,

	Options: []cmds.Option{
		// TODO longform
		cmds.StringOption("f", "The path where IPFS should be mounted\n(default is '/ipfs')"),

		// TODO longform
		cmds.StringOption("n", "The path where IPNS should be mounted\n(default is '/ipns')"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		ctx := req.Context()

		// error if we aren't running node in online mode
		if ctx.Node.Network == nil {
			return nil, errNotOnline
		}

		if err := platformFuseChecks(); err != nil {
			return nil, err
		}

		fsdir, found, err := req.Option("f").String()
		if err != nil {
			return nil, err
		}
		if !found {
			fsdir = ctx.Config.Mounts.IPFS // use default value
		}
		fsdone := mountIpfs(ctx.Node, fsdir)

		// get default mount points
		nsdir, found, err := req.Option("n").String()
		if err != nil {
			return nil, err
		}
		if !found {
			nsdir = ctx.Config.Mounts.IPNS // NB: be sure to not redeclare!
		}

		nsdone := mountIpns(ctx.Node, nsdir, fsdir)

		// wait until mounts return an error (or timeout if successful)
		var err error
		select {
		case err = <-fsdone:
		case err = <-nsdone:

		// mounted successfully, we timed out with no errors
		case <-time.After(mountTimeout):
			output := ctx.Config.Mounts
			return &output, nil
		}

		return nil, err
	},
	Type: &config.Mounts{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*config.Mounts)
			s := fmt.Sprintf("IPFS mounted at: %s\n", v.IPFS)
			s += fmt.Sprintf("IPNS mounted at: %s\n", v.IPNS)
			return []byte(s), nil
		},
	},
}

func mountIpfs(node *core.IpfsNode, fsdir string) <-chan error {
	done := make(chan error)
	log.Info("Mounting IPFS at ", fsdir)

	go func() {
		err := rofs.Mount(node, fsdir)
		done <- err
		close(done)
	}()

	return done
}

func mountIpns(node *core.IpfsNode, nsdir, fsdir string) <-chan error {
	if nsdir == "" {
		return nil
	}
	done := make(chan error)
	log.Info("Mounting IPNS at ", nsdir)

	go func() {
		err := ipns.Mount(node, nsdir, fsdir)
		done <- err
		close(done)
	}()

	return done
}

var platformFuseChecks = func() error {
	return nil
}
