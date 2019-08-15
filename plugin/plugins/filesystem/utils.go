package filesystem

import (
	"errors"
	"os"
	"strings"
)

const (
	EnvAddr = "IPFS_FS_ADDR"

	DefaultVersion  = "9P2000.L"
	DefaultProtocol = "tcp"
	DefaultAddress  = ":564"
	DefaultMSize    = 64 << 10
)

func getAddr() (proto, addr string, err error) {
	if addr = os.Getenv(EnvAddr); addr == "" {
		proto, addr = DefaultProtocol, DefaultAddress
	} else {
		pair := strings.Split(addr, ";")
		if len(pair) != 2 {
			err = errors.New("addr env-var not formated correctly")
			return
		}
		proto, addr = pair[0], pair[1]
	}
	return
}
