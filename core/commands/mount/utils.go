package fusemount

import (
	"context"
	"errors"
	"fmt"
	gopath "path"
	"strings"

	dag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
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

func (fs *FUSEIPFS) fuseReadlink(fsNode fusePath) (string, error) {

	ipldNode, err := fs.core.ResolveNode(fs.ctx, fsNode)
	if err != nil {
		return "", err
	}

	unixNode, err := unixfs.ExtractFSNode(ipldNode)
	if err != nil {
		return "", err
	}

	if unixNode.Type() != unixfs.TSymlink {
		return "", errNoLink
	}

	return string(unixNode.Data()), nil
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

func resolveKeyName(ctx context.Context, api coreiface.KeyAPI, keyString string) (coreiface.Key, error) {
	if keyString == "self" {
		return api.Self(ctx)
	}

	keys, err := api.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if keyString == key.Name() {
			return key, nil
		}
	}

	return nil, errNoKey
}

//TODO: remove this and inline publishing?
func (fs *FUSEIPFS) ipnsDelayedPublish(key coreiface.Key, node ipld.Node) error {
	oAPI, err := fs.core.WithOptions(coreoptions.Api.Offline(true))
	if err != nil {
		return err
	}

	coreTarget, err := coreiface.ParsePath(node.String())
	if err != nil {
		return err
	}

	_, err = oAPI.Name().Publish(fs.ctx, coreTarget, coreoptions.Name.Key(key.Name()), coreoptions.Name.AllowOffline(true))
	if err != nil {
		return err
	}

	//TODO: go {grace timer based on key; publish to network }
	return nil
}

//TODO: reconsider parameters
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
func emptyNode(ctx context.Context, dagAPI coreiface.APIDagService, nodeType upb.Data_DataType, filePrefix *cid.Prefix) (ipld.Node, error) {
	if nodeType == unixfs.TFile {
		eFile := dag.NodeWithData(unixfs.FilePBData(nil, 0))
		if filePrefix != nil {
			eFile.SetCidBuilder(filePrefix.WithCodec(filePrefix.GetCodec()))
		}
		if err := dagAPI.Add(ctx, eFile); err != nil {
			return nil, err
		}
		return eFile, nil
	} else if nodeType == unixfs.TDirectory {
		eDir, err := uio.NewDirectory(dagAPI).GetNode()
		if err != nil {
			return nil, err
		}
		return eDir, nil
	} else {
		return nil, errors.New("unexpected node type")
	}

}

//TODO: docs; return: key, path, error
//TODO: check if there's overlap with go-path
func ipnsSplit(path string) (string, string, error) {
	splitPath := strings.Split(path, "/")
	if len(splitPath) < 3 {
		return "", "", errInvalidPath
	}

	key := splitPath[2]
	index := strings.Index(path, key) + len(key)
	if index != len(path) {
		return key, path[index+1:], nil //strip leading '/'
	}
	return key, "", nil
}
