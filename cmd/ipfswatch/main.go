//go:build !plan9
// +build !plan9

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	commands "github.com/ipfs/kubo/commands"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	corehttp "github.com/ipfs/kubo/core/corehttp"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"

	fsnotify "github.com/fsnotify/fsnotify"
	"github.com/ipfs/go-libipfs/files"
	process "github.com/jbenet/goprocess"
	homedir "github.com/mitchellh/go-homedir"
)

var http = flag.Bool("http", false, "expose IPFS HTTP API")
var repoPath = flag.String("repo", os.Getenv("IPFS_PATH"), "IPFS_PATH to use")
var watchPath = flag.String("path", ".", "the path to watch")

func main() {
	flag.Parse()

	// precedence
	// 1. --repo flag
	// 2. IPFS_PATH environment variable
	// 3. default repo path
	var ipfsPath string
	if *repoPath != "" {
		ipfsPath = *repoPath
	} else {
		var err error
		ipfsPath, err = fsrepo.BestKnownPath()
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := run(ipfsPath, *watchPath); err != nil {
		log.Fatal(err)
	}
}

func run(ipfsPath, watchPath string) error {

	proc := process.WithParent(process.Background())
	log.Printf("running IPFSWatch on '%s' using repo at '%s'...", watchPath, ipfsPath)

	ipfsPath, err := homedir.Expand(ipfsPath)
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := addTree(watcher, watchPath); err != nil {
		return err
	}

	r, err := fsrepo.Open(ipfsPath)
	if err != nil {
		// TODO handle case: daemon running
		// TODO handle case: repo doesn't exist or isn't initialized
		return err
	}

	node, err := core.NewNode(context.Background(), &core.BuildCfg{
		Online: true,
		Repo:   r,
	})
	if err != nil {
		return err
	}
	defer node.Close()

	api, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}

	if *http {
		addr := "/ip4/127.0.0.1/tcp/5001"
		var opts = []corehttp.ServeOption{
			corehttp.GatewayOption(true, "/ipfs", "/ipns"),
			corehttp.WebUIOption,
			corehttp.CommandsOption(cmdCtx(node, ipfsPath)),
		}
		proc.Go(func(p process.Process) {
			if err := corehttp.ListenAndServe(node, addr, opts...); err != nil {
				return
			}
		})
	}

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-interrupts:
			return nil
		case e := <-watcher.Events:
			log.Printf("received event: %s", e)
			isDir, err := IsDirectory(e.Name)
			if err != nil {
				continue
			}
			switch e.Op {
			case fsnotify.Remove:
				if isDir {
					if err := watcher.Remove(e.Name); err != nil {
						return err
					}
				}
			default:
				// all events except for Remove result in an IPFS.Add, but only
				// directory creation triggers a new watch
				switch e.Op {
				case fsnotify.Create:
					if isDir {
						if err := addTree(watcher, e.Name); err != nil {
							return err
						}
					}
				}
				proc.Go(func(p process.Process) {
					file, err := os.Open(e.Name)
					if err != nil {
						log.Println(err)
						return
					}
					defer file.Close()

					st, err := file.Stat()
					if err != nil {
						log.Println(err)
						return
					}

					f, err := files.NewReaderPathFile(e.Name, file, st)
					if err != nil {
						log.Println(err)
						return
					}

					k, err := api.Unixfs().Add(node.Context(), f)
					if err != nil {
						log.Println(err)
					}
					log.Printf("added %s... key: %s", e.Name, k)
				})
			}
		case err := <-watcher.Errors:
			log.Println(err)
		}
	}
}

func addTree(w *fsnotify.Watcher, root string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return nil
		}
		isDir, err := IsDirectory(path)
		if err != nil {
			log.Println(err)
			return nil
		}
		switch {
		case isDir && IsHidden(path):
			log.Println(path)
			return filepath.SkipDir
		case isDir:
			log.Println(path)
			if err := w.Add(path); err != nil {
				return err
			}
		default:
			return nil
		}
		return nil
	})
	return err
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	return fileInfo.IsDir(), err
}

func IsHidden(path string) bool {
	path = filepath.Base(path)
	if path == "." || path == "" {
		return false
	}
	if rune(path[0]) == rune('.') {
		return true
	}
	return false
}

func cmdCtx(node *core.IpfsNode, repoPath string) commands.Context {
	return commands.Context{
		ConfigRoot: repoPath,
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, nil
		},
	}
}
