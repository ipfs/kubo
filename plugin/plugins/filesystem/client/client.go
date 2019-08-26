package p9client

import (
	gopath "path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hugelgupf/p9/p9"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/plugin/plugins/filesystem"
	logging "github.com/ipfs/go-log"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var logger logging.EventLogger = logging.Logger("9p")

func Dial(options ...Option) (*p9.Client, error) {
	ops := &Options{
		address: filesystem.DefaultListenAddress,
		msize:   filesystem.DefaultMSize,
		version: filesystem.DefaultVersion}

	for _, op := range options {
		op(ops)
	}

	if ops.address == filesystem.DefaultListenAddress {
		// expand $IPFS_PATH, using default if not exist
		target, err := config.Path("", "filesystem.9p.sock")
		if err != nil {
			return nil, err
		}
		//TODO [manet]: doesn't like drive letters
		//XXX: so for now we decap drive-spec-like paths and use the current working drive letter, relatively
		if runtime.GOOS == "windows" {
			if target, err = windowsToUnixFriendly(target); err != nil {
				return nil, err
			}
		}
		ops.address = gopath.Join("/unix", target)
	}

	ma, err := multiaddr.NewMultiaddr(ops.address)
	if err != nil {
		return nil, err
	}

	conn, err := manet.Dial(ma)
	if err != nil {
		return nil, err
	}

	return p9.NewClient(conn, filesystem.DefaultMSize, filesystem.DefaultVersion)
	/*
		client, err := p9.NewClient(conn, filesystem.DefaultMSize, filesystem.DefaultVersion)
		if err != nil {
			return nil, err
		}
		logger.Infof("Connected to server supporting version:\n%v\n\n", client.Version())

		rootRef, err := client.Attach("")
		if err != nil {
			return nil, err
		}
		logger.Debugf("Attached to root:\n%#v\n\n", rootRef)
	*/
}

func ReadDir(path string, fsRef p9.File, offset uint64) ([]p9.Dirent, error) {
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(components) == 1 && components[0] == "" {
		components = nil
	}

	qids, targetRef, err := fsRef.Walk(components)
	if err != nil {
		return nil, err
	}
	logger.Debugf("walked to %q :\nQIDs:%v, FID:%v\n\n", path, qids, targetRef)

	if _, _, err = targetRef.Open(0); err != nil {
		return nil, err
	}

	ents, err := targetRef.Readdir(offset, ^uint32(0))
	if err != nil {
		return nil, err
	}

	logger.Debugf("%q Readdir:\n[%d]%v\n\n", path, len(ents), ents)
	if err = targetRef.Close(); err != nil {
		return nil, err
	}
	logger.Debugf("%q closed:\n%#v\n\n", path, targetRef)

	return ents, nil
}

/* TODO: rework
func Open(path string, fsRef p9.File) (p9.File, error) {
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(components) == 1 && components[0] == "" {
		components = nil
	}

	_, targetRef, err := fsRef.Walk(nil)
	if err != nil {
		return nil, err
	}

	_, _, attr, err := targetRef.GetAttr(p9.AttrMask{Size: true})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Getattr for %q :\n%v\n\n", path, attr)

	refQid, ioUnit, err := targetRef.Open(0)
	if err != nil {
		return nil, err
	}

}

func Read(path string, openedRef p9.File) {
	components := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(components) == 1 && components[0] == "" {
		components = nil
	}

	_, targetRef, err := fsRef.Walk(components)
	if err != nil {
		return nil, err
	}

	_, _, attr, err := targetRef.GetAttr(p9.AttrMask{Size: true})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Getattr for %q :\n%v\n\n", path, attr)

	refQid, ioUnit, err := targetRef.Open(0)
	if err != nil {
		return nil, err
	}
	logger.Debugf("%q Opened:\nQID:%v, iounit:%v\n\n", path, refQid, ioUnit)

	buf := make([]byte, attr.Size)
	readBytes, err := targetRef.ReadAt(buf, 0)
	if err != nil {
		return nil, err
	}

	logger.Debugf("%q Read:\n[%d bytes]\n%s\n\n", path, readBytes, buf)
	if err = targetRef.Close(); err != nil {
		return nil, err
	}

	logger.Debugf("%q closed:\n%#v\n\n", path, targetRef)
}
*/

func windowsToUnixFriendly(target string) (string, error) {
	if !filepath.IsAbs(target) {
		var err error
		if target, err = filepath.Abs(target); err != nil {
			return target, err
		}
	}

	//TODO [manet]: doesn't like drive letters
	//XXX: so for now we decap drive-spec-like paths and use the current working drive letter, relatively
	if len(target) > 3 && target[1] == ':' {
		target = target[3:]
	}
	return filepath.ToSlash(target), nil
}
