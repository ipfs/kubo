package lib

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	config "github.com/ipfs/go-ipfs-config"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	logging "github.com/ipfs/go-log"
        loggables "github.com/libp2p/go-libp2p-loggables"
)

const (
	waitforDaemonInternal = 20 * time.Second
)

type ipfsDaemon struct {
	// Ipfs daemon context
	ctx    context.Context

	// Channel to deliver daemon environment
	envCh  chan *oldcmds.Context

	// Channel to deliver daemon internal error
	errCh  chan error

	// Ipfs daemon environment
	env    *oldcmds.Context

	// Daemon ready flag
	ready  bool

	// Cancle function
	cancel context.CancelFunc
}

var (
	daemon *ipfsDaemon
	mutex  sync.Mutex
)

var (
	ErrNotInit  = errors.New("Not init ipfs")
	ErrRenit    = errors.New("Reinit ipfs module")
	ErrNotReady = errors.New("Ipfs daemon not ready")
	ErrHasReady = errors.New("Ipfs daemon has been running")
)

func (d *ipfsDaemon) start() (error, <-chan error) {
	go command(d.ctx, daemonCommand, d.envCh, d.errCh)

	// block until receive daemon env
	d.env = <-d.envCh

	// In this condation some error happens
	if d.env == nil {
		err := <-d.errCh
		fmt.Printf("Start ipfs daemon error:%v\n", err)
		return err, d.errCh
	}

	// Wait for a while to ensure daemon is launched successfully.
	select {
	case err := <-d.errCh:
		fmt.Printf("Start ipfs daemon error:%v\n", err)
		return err, d.errCh
	case <-time.After(waitforDaemonInternal):
		fmt.Println("Start ipfs daemon successfully.")
	}

	d.ready = true
	return nil, d.errCh
}

func (d *ipfsDaemon) stop() {
	d.cancel()

	err := <-d.errCh
	if err == ErrNormalExit {
		fmt.Println("Ipfs daemon exit normally")
	} else {
		fmt.Printf("Ipfs daemon exit error:%v\n", err)
	}

	d.ready = false
	close(d.envCh)
	close(d.errCh)
}

func (d *ipfsDaemon) config() (*config.Config, error) {
        return d.env.GetConfig()
}

func (d *ipfsDaemon) node() (*core.IpfsNode, error) {
        return d.env.GetNode()
}

func (d *ipfsDaemon) api() (coreiface.CoreAPI, error) {
        return d.env.GetAPI()
}

func InitIpfs(repoPath string) error {
	mutex.Lock()
	defer mutex.Unlock()

	if daemon != nil && daemon.ready {
		return ErrRenit
	}

	if err := setupRepoPath(repoPath); err != nil {
		return err
	}

	rand.Seed(time.Now().UnixNano())

	daemon = &ipfsDaemon{
		envCh:    make(chan *oldcmds.Context, 1),
		errCh:    make(chan error, 1),
		ready:    false,
	}

	ctx := logging.ContextWithLoggable(context.Background(), loggables.Uuid("session"))
	daemon.ctx, daemon.cancel = context.WithCancel(ctx)

	return nil
}

func setupRepoPath(repoPath string) error {
	var err error

	if repoPath == "" {
		if repoPath, err = fsrepo.BestKnownPath(); err != nil {
			return err
		}
	}

	// Set IPFS_PATH environment varbile
	if err = os.Setenv("IPFS_PATH", repoPath); err != nil {
		return err
	}

        return nil
}

func StartDaemon() (error, <-chan error) {
	mutex.Lock()
	defer mutex.Unlock()

	if daemon == nil {
		return ErrNotInit, nil
	}

	if daemon.ready {
		return ErrHasReady, nil
	}

	return daemon.start()
}

func StopDaemon() {
	mutex.Lock()
	defer mutex.Unlock()

	if daemon == nil || !daemon.ready {
		return
	}

	daemon.stop()
}

func Config() (*config.Config, error) {
        mutex.Lock()
        defer mutex.Unlock()

        if daemon == nil {
                return nil, ErrNotInit
        }

	if !daemon.ready {
		return nil, ErrNotReady
	}

        return daemon.config()
}

func Node() (*core.IpfsNode, error) {
        mutex.Lock()
        defer mutex.Unlock()

        if daemon == nil {
                return nil, ErrNotInit
        }

        if !daemon.ready {
                return nil, ErrNotReady
        }

        return daemon.node()
}

func API() (coreiface.CoreAPI, error) {
        mutex.Lock()
        defer mutex.Unlock()

        if daemon == nil {
                return nil, ErrNotInit
        }

        if !daemon.ready {
                return nil, ErrNotReady
        }

	return daemon.api()
}
