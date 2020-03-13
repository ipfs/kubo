package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	ipfs "github.com/ipfs/go-ipfs/lib"
	util "github.com/ipfs/go-ipfs/lib/util"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var (
	repoDir = "tau-ipfs"
)

func main() {

	intrh, ctx := util.SetupInterruptHandler(context.Background())
	defer intrh.Close()

	var err      error
	var errCh    <-chan error
	var wg       sync.WaitGroup
	var repoPath string

	repoPath = os.Getenv("HOME") + "/" + repoDir
	fmt.Printf("repo path is:%s\n", repoPath)

	if err = ipfs.InitIpfs(repoPath); err != nil {
		fmt.Printf("init ipfs error:%v\n", err)
		os.Exit(1)
	}

	// Start ipfs daemon
	err, errCh = ipfs.StartDaemon();
	if err != nil {
		fmt.Printf("start daemon error:%v\n", err)
		os.Exit(1)
	}

	// Here ipfs daemon is running, so run some test cases.
	testCoreAPI()

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case derr := <-errCh:
			if derr != ipfs.ErrNormalExit {
				fmt.Printf("ipfs daemon internal error:%v\n", derr)
				ipfs.StopDaemon()
			} else {
				fmt.Println("ipfs daemon exit normally")
			}
		case <-ctx.Done():
			ipfs.StopDaemon()
		}
	}()

	wg.Wait()
}

func testCoreAPI() {
	var api coreiface.CoreAPI
	var err error

	fmt.Println("Coreapi test is starting...")

	if api, err = ipfs.API(); err != nil {
		fmt.Printf("get api error:%v\n", err)
		fmt.Println("Coreapi test failed.")
		return
	}

        keyAPI := api.Key()
        id, _ := keyAPI.Self(context.Background())
        fmt.Printf("IPFS Node id:%s\n", id.ID())

	fmt.Println("Coreapi test passed")
}
