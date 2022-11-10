package final

import (
	"context"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"github.com/urfave/cli/v2"
)

func createReleaselog(cCtx *cli.Context) error {
	ctx := context.Background()

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
			return err
	}
	defer client.Close()

	// get reference to the local project

	path, err := os.Getwd()
	if err != nil {
		return err
	}
	src := client.Host().Directory(filepath.Dir(path))

	// create empty directory to put build outputs
	outputs := client.Directory()

	// get `golang` image
	golang := client.Container().From("golang:1.19")


	golang = golang.Exec(dagger.ContainerExecOpts{
		Args: []string{"apt", "update"},
	})

	golang = golang.Exec(dagger.ContainerExecOpts{
		Args: []string{"apt", "install", "-y", "zsh", "zplug", "jq"},
	})

	golang = golang.WithMountedDirectory("/go/src/github.com/ipfs/kubo", src).WithWorkdir("/go/src/github.com/ipfs/kubo")

	golang = golang.Exec(dagger.ContainerExecOpts{
		Args: []string{"bin/mkreleaselog"},
	})

	golang = golang.Exec(dagger.ContainerExecOpts{
		Args: []string{"mkdir", "/out"},
	})

	golang = golang.Exec(dagger.ContainerExecOpts{
		Args: []string{"bin/mkreleaselog"},
		RedirectStdout: "/out/releaselog.md",
	})

	outputs = outputs.WithFile("releaselog.md", golang.File("/out/releaselog.md"))

	_, err = outputs.Export(ctx, ".")
	if err != nil {
			return err
	}
	return nil
}

var CreateReleaselog = &cli.Command{
	Name:   "create-releaselog",
	Action: createReleaselog,
}
