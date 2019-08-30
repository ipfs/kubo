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

var logger logging.EventLogger = logging.Logger("9P")

func Dial(options ...Option) (*p9.Client, error) {
	ops := &Options{
		address: filesystem.DefaultListenAddress,
		msize:   filesystem.DefaultMSize,
		version: filesystem.DefaultVersion}

	for _, op := range options {
		op(ops)
	}

	//TODO: this should probably be the callers responsibility
	if ops.address == filesystem.DefaultListenAddress {
		var err error
		if ops.address, err = expandDefault(); err != nil {
			return nil, err
		}
	}

	ma, err := multiaddr.NewMultiaddr(ops.address)
	if err != nil {
		return nil, err
	}

	//TODO [investigate;who's bug] on Windows, dialing a unix domain socket that doesn't exist will create it
	conn, err := manet.Dial(ma)
	if err != nil {
		return nil, err
	}

	return p9.NewClient(conn, filesystem.DefaultMSize, filesystem.DefaultVersion)
}

func expandDefault() (string, error) {
	_, sockName := gopath.Split(filesystem.DefaultListenAddress)
	target, err := config.Path("", sockName)
	if err != nil {
		return target, err
	}

	if !filepath.IsAbs(target) {
		if target, err = filepath.Abs(target); err != nil {
			return target, err
		}
	}

	if runtime.GOOS == "windows" {
		//TODO [manet]: doesn't like drive letters
		//XXX: so for now we decap drive-spec-like paths and use the current working drive letter, relatively
		if len(target) > 2 && target[1] == ':' {
			target = target[2:]
		}
		target = filepath.ToSlash(target)
	}

	target = gopath.Join("/unix", target)
	return target, nil
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
