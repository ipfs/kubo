package commands

import (
	"os"
	"path"
	"runtime"

	cmds "github.com/ipfs/go-ipfs/commands"

	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	psud "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/shirou/gopsutil/disk"
	psum "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/shirou/gopsutil/mem"
)

var sysDiagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "prints out system diagnostic information.",
		ShortDescription: `
Prints out information about your computer to aid in easier debugging.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		info := make(map[string]interface{})
		err := runtimeInfo(info)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = envVarInfo(info)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = diskSpaceInfo(info)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = memInfo(info)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = netInfo(info)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(info)
	},
}

func runtimeInfo(out map[string]interface{}) error {
	rt := make(map[string]interface{})
	rt["os"] = runtime.GOOS
	rt["arch"] = runtime.GOARCH
	rt["compiler"] = runtime.Compiler
	rt["version"] = runtime.Version()
	rt["numcpu"] = runtime.NumCPU()
	rt["gomaxprocs"] = runtime.GOMAXPROCS(0)
	rt["numgoroutines"] = runtime.NumGoroutine()

	out["runtime"] = rt
	return nil
}

func envVarInfo(out map[string]interface{}) error {
	ev := make(map[string]interface{})
	ev["GOPATH"] = os.Getenv("GOPATH")
	ev["IPFS_PATH"] = os.Getenv("IPFS_PATH")

	out["environment"] = ev
	return nil
}

func ipfsPath() string {
	p := os.Getenv("IPFS_PATH")
	if p == "" {
		p = path.Join(os.Getenv("HOME"), ".ipfs")
	}
	return p
}

func diskSpaceInfo(out map[string]interface{}) error {
	di := make(map[string]interface{})
	dinfo, err := psud.DiskUsage(ipfsPath())
	if err != nil {
		return err
	}

	di["fstype"] = dinfo.Fstype
	di["total_space"] = dinfo.Total
	di["used_space"] = dinfo.Used
	di["free_space"] = dinfo.Free

	out["diskinfo"] = di
	return nil
}

func memInfo(out map[string]interface{}) error {
	m := make(map[string]interface{})
	swap, err := psum.SwapMemory()
	if err != nil {
		return err
	}

	virt, err := psum.VirtualMemory()
	if err != nil {
		return err
	}

	m["swap"] = swap
	m["virt"] = virt
	out["memory"] = m
	return nil
}

func netInfo(out map[string]interface{}) error {
	n := make(map[string]interface{})
	addrs, err := manet.InterfaceMultiaddrs()
	if err != nil {
		return err
	}

	var straddrs []string
	for _, a := range addrs {
		straddrs = append(straddrs, a.String())
	}

	n["interface_addresses"] = straddrs
	out["net"] = n
	return nil
}
