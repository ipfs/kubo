package fusemount

import (
	"context"
	"fmt"
	gopath "path"
	"strings"
	"time"

	dag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	uio "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs/io"
	upb "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs/pb"
)

//const mutableFlags = fuse.O_WRONLY | fuse.O_RDWR | fuse.O_APPEND | fuse.O_CREAT | fuse.O_TRUNC

func platformException(path string) bool {
	//TODO: add detection for common platform path patterns to avoid flooding error log
	/*
		macos:
			.DS_Store
		NT:
			Autorun.inf
			desktop.ini
			Thumbs.db
			*.exe.Config
			*.exe.lnk
			...
	*/
	//TODO: move this to a build constraint file filter_windows.go
	switch strings.ToLower(gopath.Base(path)) {
	case "autorun.inf", "desktop.ini", "folder.jpg", "folder.gif", "thumbs.db":
		return true
	}
	return strings.HasSuffix(path, ".exe.Config")
}

func unixAddChild(ctx context.Context, dagSrv coreiface.APIDagService, rootNode ipld.Node, path string, node ipld.Node) (ipld.Node, error) {
	rootDir, err := uio.NewDirectoryFromNode(dagSrv, rootNode)
	if err != nil {
		return nil, err
	}

	err = rootDir.AddChild(ctx, path, node)
	if err != nil {
		return nil, err
	}

	newRoot, err := rootDir.GetNode()
	if err != nil {
		return nil, err
	}

	if err := dagSrv.Add(ctx, newRoot); err != nil {
		return nil, err
	}
	return newRoot, nil
}

func resolveIpns(ctx context.Context, path string, core coreiface.CoreAPI) (ipld.Node, error) {
	pathKey, subPath := ipnsSplit(path)

	oAPI, err := core.WithOptions(coreoptions.Api.Offline(true))
	if err != nil {
		return nil, err
	}

	var nameAPI coreiface.NameAPI
	globalPath := path
	coreKey, err := resolveKeyName(ctx, oAPI.Key(), pathKey)
	switch err {
	case nil: // locally owned keys are resolved offline
		globalPath = gopath.Join(coreKey.Path().String(), subPath)
		nameAPI = oAPI.Name()

	case errNoKey: // paths without owned keys are valid, but looked up via network instead of locally
		nameAPI = core.Name()

	case ds.ErrNotFound: // API conflict; A key exists, but holds no value (generated but not published to)
		return nil, fmt.Errorf("IPNS key %q has no value: %s", pathKey, err)
	default:
		return nil, err
	}

	resolvedPath, err := nameAPI.Resolve(ctx, globalPath, coreoptions.Name.Cache(true))
	if err != nil {
		return nil, err
	}

	// target resolution is done with online core regardless of ownership
	// (local node may not have target path, but not target/key data)
	return core.ResolveNode(ctx, resolvedPath)
}

func resolveKeyName(ctx context.Context, api coreiface.KeyAPI, keyString string) (coreiface.Key, error) {
	if keyString == "self" {
		return api.Self(ctx)
	}

	keys, err := api.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if keyString == key.Name() || keyString == key.Id() {
			return key, nil
		}
	}

	return nil, errNoKey
}

//TODO: remove this and inline publishing?
func ipnsDelayedPublish(ctx context.Context, key coreiface.Key, node ipld.Node) error {
	oAPI, err := fs.core.WithOptions(coreoptions.Api.Offline(true))
	if err != nil {
		return err
	}

	coreTarget, err := coreiface.ParsePath(node.String())
	if err != nil {
		return err
	}

	_, err = oAPI.Name().Publish(ctx, coreTarget, coreoptions.Name.Key(key.Name()), coreoptions.Name.AllowOffline(true))
	if err != nil {
		return err
	}

	//TODO: go {grace timer based on key; publish to network }
	return nil
}

func ipnsPublisher(keyName string, nameAPI coreiface.NameAPI) func(context.Context, cid.Cid) error {
	return func(ctx context.Context, rootCid cid.Cid) error {
		//log.Errorf("publish request; key:%q cid:%q", keyName, rootCid)
		_, err := nameAPI.Publish(ctx, coreiface.IpfsPath(rootCid), coreoptions.Name.Key(keyName), coreoptions.Name.AllowOffline(true))
		//log.Errorf("published %q to %q", ent.Value(), ent.Name())
		return err
	}
}

//TODO: do this on initialization of IPNS keys; embed in struct
func (fs FUSEIPFS) ipnsMFSSplit(path string) (*mfs.Root, string, error) {
	keyName, subPath, _ := ipnsSplit(path)
	keyRoot := fs.nameRoots[keyName]
	if keyRoot == nil {
		return nil, "", fmt.Errorf("mfs root for key %s not initialized", keyName)
	}
	return keyRoot, subPath, nil
}

//XXX
func emptyNode(ctx context.Context, dagAPI coreiface.APIDagService, nodeType upb.Data_DataType) (ipld.Node, error) {
	switch nodeType {
	case unixfs.TFile:
		eFile := dag.NodeWithData(unixfs.FilePBData(nil, 0))
		if err := dagAPI.Add(ctx, eFile); err != nil {
			return nil, err
		}
		return eFile, nil
	case unixfs.TDirectory:
		eDir, err := uio.NewDirectory(dagAPI).GetNode()
		if err != nil {
			return nil, err
		}
		return eDir, nil
	default:
		return nil, errUnexpected
	}
}

//TODO: docs; return: key, path
func ipnsSplit(path string) (string, string) {
	splitPath := strings.Split(path, "/")
	key := splitPath[2]
	index := strings.Index(path, key) + len(key)
	return key, path[index:]
}

func deriveCallContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, callTimeout)
}

type timerContextActual struct {
	context.Context
	cancel context.CancelFunc
	timer  time.Timer
	grace  time.Duration
}

func (tctx *timerContextActual) Reset() {
	if !tctx.timer.Stop() {
		<-tctx.timer.C
	}
	tctx.timer.Reset(tctx.grace)
}

type timerContext interface {
	context.Context
	Reset()
	Cancel()
}

func deriveTimerContext(ctx context.Context, grace time.Duration) timerContext {
	asyncContext, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(grace, cancel())
	tctx := timerContextActual{context.Context: asyncContext, cancel: cancel, grace: grace, timer: timer}

	return tctx
}

func checkAPIKeystore(ctx context.Context, keyAPI coreiface.KeyAPI, coreKey coreiface.Key) error {
	coreKey, err := resolveKeyName(ctx, keyAPI, coreKey.Name())
	switch err {
	default:
		return err
	case errNoKey:
		return errNoKey
	case nil: // path contains key we own
		if api, arg := coreKey.ID(), coreKey.ID(); api != arg {
			return fmt.Errorf("key ID conflict, daemon:%q File System:%q", api, arg)
		}
		return nil
	}
}

func mfsFromKey(ctx context.Context, coreKey coreiface.Key, core coreiface.CoreAPI) (*mfs.Root, error) {
	ipldNode, err := core.ResolveNode(ctx, coreKey.Path()) //TODO: offline this
	if err != nil {
		return nil, err
	}

	pbNode, ok := ipldNode.(*dag.ProtoNode)
	if !ok {
		return nil, fmt.Errorf("key %q has incompatible type %T", coreKey.Name(), ipldNode)
	}

	return mfs.NewRoot(ctx, core.Dag(), pbNode, ipnsPublisher(coreKey.Name(), core.Name()))
}

func initOrGetMFSKeyRoot(ctx context.Context, keyName string, nr nameRootIndex) (*mfs.Root, error) {
	nr.Lock()
	defer nr.Unlock()
	mroot, err := nr.Request(keyName)
	switch err {
	case errNotInitialized:
		//init mfs
		nn.core.ResolveNode(ctx, key.Path()) //TODO: offline this?
		pbNode, ok := keyNode.(*dag.ProtoNode)
		if !ok {
			return nil, fmt.Errorf("key %q has incompatible type %T", keyName, keyNode)
		}

		//continue modifying this struct
		keyRoot, err := mfs.NewRoot(ctx, nn.core.Dag(), pbNode, ipnsPublisher(key.Name(), nn.core.Name()))
		//nn.core.Name().Resolve(ctx, key.Path.Stringname string, opts ...options.NameResolveOption) (Path, error)

		//resolve key to node, node to key io

		//register()
	default:
		return nil, err
	}

}

/*
func lockUp(path string, lookup lookupFn) (unlock func()) {
	components := strings.Split(path, "/")

	nodeLocks := make([]func(), len(components))
	var wg sync.WaitGroup
	wg.Add(len(components))

	for i := len(components); i >= 0; i-- {
		go func(i int) {
			defer wg.Done()
			node, err := lookup(components[i])
			if err != nil {
				return
			}
			node.Lock()
			nodeLocks = append(nodeLocks, node.Unlock)
		}(i)
	}
	for curPath := gopath.Dir(path); curPath != "/"; {
	}

	unlock = func() {
		for _, unlock := range nodeLocks {
			unlock()
		}
	}
	return
}
*/
/*
func decap(string, subsystemType typeToken) string {
    switch subsystemType
    /ipns/key/whatever -> /whatever
    /ipfs/Qm... -> /Qm...
}
*/

//TODO: check how MFS uses this context
func ipnsToMFSRoot(ctx context.Context, path string, core coreiface.CoreAPI) (*mfs.Root, error) {

	keyName, _ := ipnsSplit(path)

	ipldNode, err := resolveIpns(ctx, path, core)
	if err != nil {
		return nil, err
	}
	pbNode, ok := ipldNode.(*dag.ProtoNode)
	if !ok {
		return nil, fmt.Errorf("key %q points to incompatible type %T", keyName, ipldNode)
	}
	mroot, err := mfs.NewRoot(ctx, core.Dag(), pbNode, ipnsPublisher(keyName, core.Name()))
	if err != nil {
		return nil, err
	}
	return mroot, nil
}
