package newnd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/jbenet/goprocess"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/keystore"
	"github.com/ipfs/go-ipfs/repo"
)

func memRepo() repo.Repo {
	c := config.Config{}
	// c.Identity = ident //TODO, probably
	c.Experimental.FilestoreEnabled = true

	ds := datastore.NewMapDatastore()
	return &repo.Mock{
		C: c,
		D: syncds.MutexWrap(ds),
		K: keystore.NewMemKeystore(),
		F: filestore.NewFileManager(ds, filepath.Dir(os.TempDir())),
	}
}

// copied from old node pkg

// baseProcess creates a goprocess which is closed when the lifecycle signals it to stop
func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}
