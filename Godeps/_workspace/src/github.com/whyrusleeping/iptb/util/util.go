package iptbutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	serial "github.com/ipfs/go-ipfs/repo/fsrepo/serialize"

	manet "gx/ipfs/QmYVqhVfbK4BKvbW88Lhm26b3ud14sTBvcm1H7uWUx1Fkp/go-multiaddr-net"
	ma "gx/ipfs/QmcobAGsCjYt5DXoq9et9L8yR8er7o7Cu3DTvpaq12jYSz/go-multiaddr"
)

var setupOpt = func(cmd *exec.Cmd) {}

// GetNumNodes returns the number of testbed nodes configured in the testbed directory
func GetNumNodes() int {
	for i := 0; i < 2000; i++ {
		_, err := os.Stat(IpfsDirN(i))
		if os.IsNotExist(err) {
			return i
		}
	}
	panic("i dont know whats going on")
}

func TestBedDir() string {
	tbd := os.Getenv("IPTB_ROOT")
	if len(tbd) != 0 {
		return tbd
	}

	home := os.Getenv("HOME")
	if len(home) == 0 {
		panic("could not find home")
	}

	return path.Join(home, "testbed")
}

func IpfsDirN(n int) string {
	return path.Join(TestBedDir(), fmt.Sprint(n))
}

type InitCfg struct {
	Count     int
	Force     bool
	Bootstrap string
	PortStart int
	Mdns      bool
	Utp       bool
	Override  string
}

func (c *InitCfg) swarmAddrForPeer(i int) string {
	str := "/ip4/0.0.0.0/tcp/%d"
	if c.Utp {
		str = "/ip4/0.0.0.0/udp/%d/utp"
	}

	if c.PortStart == 0 {
		return fmt.Sprintf(str, 0)
	}
	return fmt.Sprintf(str, c.PortStart+i)
}

func (c *InitCfg) apiAddrForPeer(i int) string {
	if c.PortStart == 0 {
		return "/ip4/127.0.0.1/tcp/0"
	}
	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", c.PortStart+1000+i)
}

func YesNoPrompt(prompt string) bool {
	var s string
	for {
		fmt.Println(prompt)
		fmt.Scanf("%s", &s)
		switch s {
		case "y", "Y":
			return true
		case "n", "N":
			return false
		}
		fmt.Println("Please press either 'y' or 'n'")
	}
}

func IpfsInit(cfg *InitCfg) error {
	p := IpfsDirN(0)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		if !cfg.Force && !YesNoPrompt("testbed nodes already exist, overwrite? [y/n]") {
			return nil
		}
		err := os.RemoveAll(TestBedDir())
		if err != nil {
			return err
		}
	}
	wait := sync.WaitGroup{}
	for i := 0; i < cfg.Count; i++ {
		wait.Add(1)
		go func(v int) {
			defer wait.Done()
			dir := IpfsDirN(v)
			err := os.MkdirAll(dir, 0777)
			if err != nil {
				log.Println("ERROR: ", err)
				return
			}

			cmd := exec.Command("ipfs", "init", "-b=1024")
			cmd.Env = append(cmd.Env, "IPFS_PATH="+dir)
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Println("ERROR: ", err)
				log.Println(string(out))
			}
		}(i)
	}
	wait.Wait()

	// Now setup bootstrapping
	switch cfg.Bootstrap {
	case "star":
		err := starBootstrap(cfg)
		if err != nil {
			return err
		}
	case "none":
		err := clearBootstrapping(cfg)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognized bootstrapping option: %s", cfg.Bootstrap)
	}

	/*
		if cfg.Override != "" {
			err := ApplyConfigOverride(cfg)
			if err != nil {
				return err
			}
		}
	*/

	return nil
}

func ApplyConfigOverride(cfg *InitCfg) error {
	fir, err := os.Open(cfg.Override)
	if err != nil {
		return err
	}
	defer fir.Close()

	var configs map[string]interface{}
	err = json.NewDecoder(fir).Decode(&configs)
	if err != nil {
		return err
	}

	for i := 0; i < cfg.Count; i++ {
		err := applyOverrideToNode(configs, i)
		if err != nil {
			return err
		}
	}

	return nil
}

func applyOverrideToNode(ovr map[string]interface{}, node int) error {
	for k, v := range ovr {
		_ = k
		switch v.(type) {
		case map[string]interface{}:
		default:
		}

	}

	panic("not implemented")
}

func starBootstrap(icfg *InitCfg) error {
	// '0' node is the bootstrap node
	cfgpath := path.Join(IpfsDirN(0), "config")
	bcfg, err := serial.Load(cfgpath)
	if err != nil {
		return err
	}
	bcfg.Bootstrap = nil
	bcfg.Addresses.Swarm = []string{icfg.swarmAddrForPeer(0)}
	bcfg.Addresses.API = icfg.apiAddrForPeer(0)
	bcfg.Addresses.Gateway = ""
	bcfg.Discovery.MDNS.Enabled = icfg.Mdns
	err = serial.WriteConfigFile(cfgpath, bcfg)
	if err != nil {
		return err
	}

	for i := 1; i < icfg.Count; i++ {
		cfgpath := path.Join(IpfsDirN(i), "config")
		cfg, err := serial.Load(cfgpath)
		if err != nil {
			return err
		}

		ba := fmt.Sprintf("%s/ipfs/%s", bcfg.Addresses.Swarm[0], bcfg.Identity.PeerID)
		ba = strings.Replace(ba, "0.0.0.0", "127.0.0.1", -1)
		cfg.Bootstrap = []string{ba}
		cfg.Addresses.Gateway = ""
		cfg.Discovery.MDNS.Enabled = icfg.Mdns
		cfg.Addresses.Swarm = []string{
			icfg.swarmAddrForPeer(i),
		}
		cfg.Addresses.API = icfg.apiAddrForPeer(i)
		err = serial.WriteConfigFile(cfgpath, cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func clearBootstrapping(icfg *InitCfg) error {
	for i := 0; i < icfg.Count; i++ {
		cfgpath := path.Join(IpfsDirN(i), "config")
		cfg, err := serial.Load(cfgpath)
		if err != nil {
			return err
		}

		cfg.Bootstrap = nil
		cfg.Addresses.Gateway = ""
		cfg.Addresses.Swarm = []string{icfg.swarmAddrForPeer(i)}
		cfg.Addresses.API = icfg.apiAddrForPeer(i)
		cfg.Discovery.MDNS.Enabled = icfg.Mdns
		err = serial.WriteConfigFile(cfgpath, cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func IpfsPidOf(n int) (int, error) {
	dir := IpfsDirN(n)
	b, err := ioutil.ReadFile(path.Join(dir, "daemon.pid"))
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(string(b))
}

func KillNode(i int) error {
	pid, err := IpfsPidOf(i)
	if err != nil {
		return fmt.Errorf("error killing daemon %d: %s", i, err)
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error killing daemon %d: %s", i, err)
	}
	err = p.Kill()
	if err != nil {
		return fmt.Errorf("error killing daemon %d: %s\n", i, err)
	}

	p.Wait()

	err = os.Remove(path.Join(IpfsDirN(i), "daemon.pid"))
	if err != nil {
		return fmt.Errorf("error removing pid file for daemon %d: %s\n", i, err)
	}

	return nil
}

func IpfsKillAll() error {
	n := GetNumNodes()
	for i := 0; i < n; i++ {
		err := KillNode(i)
		if err != nil {
			return err
		}
	}
	return nil
}

func envForDaemon(n int) []string {
	envs := os.Environ()
	npath := "IPFS_PATH=" + IpfsDirN(n)
	for i, e := range envs {
		p := strings.Split(e, "=")
		if p[0] == "IPFS_PATH" {
			envs[i] = npath
			return envs
		}
	}

	return append(envs, npath)
}

func IpfsStart(waitall bool) error {
	var addrs []string
	n := GetNumNodes()
	for i := 0; i < n; i++ {
		dir := IpfsDirN(i)
		cmd := exec.Command("ipfs", "daemon")
		cmd.Dir = dir
		cmd.Env = envForDaemon(i)

		setupOpt(cmd)

		stdout, err := os.Create(path.Join(dir, "daemon.stdout"))
		if err != nil {
			return err
		}

		stderr, err := os.Create(path.Join(dir, "daemon.stderr"))
		if err != nil {
			return err
		}

		cmd.Stdout = stdout
		cmd.Stderr = stderr

		err = cmd.Start()
		if err != nil {
			return err
		}
		pid := cmd.Process.Pid

		fmt.Printf("Started daemon %d, pid = %d\n", i, pid)
		err = ioutil.WriteFile(path.Join(dir, "daemon.pid"), []byte(fmt.Sprint(pid)), 0666)
		if err != nil {
			return err
		}

		// Make sure node 0 is up before starting the rest so
		// bootstrapping works properly
		cfg, err := serial.Load(path.Join(IpfsDirN(i), "config"))
		if err != nil {
			return err
		}

		maddr := ma.StringCast(cfg.Addresses.API)
		_, addr, err := manet.DialArgs(maddr)
		if err != nil {
			return err
		}

		addrs = append(addrs, addr)

		err = waitOnAPI(cfg.Identity.PeerID, i)
		if err != nil {
			return err
		}
	}
	if waitall {
		for i := 0; i < n; i++ {
			err := waitOnSwarmPeers(i)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func waitOnAPI(peerid string, nnum int) error {
	for i := 0; i < 50; i++ {
		err := tryAPICheck(peerid, nnum)
		if err == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 200)
	}
	return fmt.Errorf("node %d failed to come online in given time period", nnum)
}

func GetNodesAPIAddr(nnum int) (string, error) {
	addrb, err := ioutil.ReadFile(path.Join(IpfsDirN(nnum), "api"))
	if err != nil {
		return "", err
	}

	maddr, err := ma.NewMultiaddr(string(addrb))
	if err != nil {
		fmt.Println("error parsing multiaddr: ", err)
		return "", err
	}

	_, addr, err := manet.DialArgs(maddr)
	if err != nil {
		fmt.Println("error on multiaddr dialargs: ", err)
		return "", err
	}
	return addr, nil
}

func tryAPICheck(peerid string, nnum int) error {
	addr, err := GetNodesAPIAddr(nnum)
	if err != nil {
		return err
	}

	resp, err := http.Get("http://" + addr + "/api/v0/id")
	if err != nil {
		return err
	}

	out := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return fmt.Errorf("liveness check failed: %s", err)
	}

	id, ok := out["ID"]
	if !ok {
		return fmt.Errorf("liveness check failed: ID field not present in output")
	}

	idstr := id.(string)
	if idstr != peerid {
		return fmt.Errorf("liveness check failed: unexpected peer at endpoint")
	}

	return nil
}

func waitOnSwarmPeers(nnum int) error {
	addr, err := GetNodesAPIAddr(nnum)
	if err != nil {
		return err
	}

	for i := 0; i < 50; i++ {
		resp, err := http.Get("http://" + addr + "/api/v0/swarm/peers")
		if err == nil {
			out := make(map[string]interface{})
			err := json.NewDecoder(resp.Body).Decode(&out)
			if err != nil {
				return fmt.Errorf("liveness check failed: %s", err)
			}

			peers := out["Strings"].([]interface{})
			if len(peers) == 0 {
				time.Sleep(time.Millisecond * 200)
				continue
			}

			return nil
		}
		time.Sleep(time.Millisecond * 200)
	}
	return fmt.Errorf("node at %s failed to bootstrap in given time period", addr)
}

// GetPeerID reads the config of node 'n' and returns its peer ID
func GetPeerID(n int) (string, error) {
	cfg, err := serial.Load(path.Join(IpfsDirN(n), "config"))
	if err != nil {
		return "", err
	}
	return cfg.Identity.PeerID, nil
}

// IpfsShell sets up environment variables for a new shell to more easily
// control the given daemon
func IpfsShell(n int) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return fmt.Errorf("couldnt find shell!")
	}

	dir := IpfsDirN(n)
	nenvs := []string{"IPFS_PATH=" + dir}

	nnodes := GetNumNodes()
	for i := 0; i < nnodes; i++ {
		peerid, err := GetPeerID(i)
		if err != nil {
			return err
		}
		nenvs = append(nenvs, fmt.Sprintf("NODE%d=%s", i, peerid))
	}
	nenvs = append(os.Environ(), nenvs...)

	return syscall.Exec(shell, []string{shell}, nenvs)
}

func ConnectNodes(from, to int) error {
	if from == to {
		// skip connecting to self..
		return nil
	}
	fmt.Printf("connecting %d -> %d\n", from, to)
	cmd := exec.Command("ipfs", "id", "-f", "<addrs>")
	cmd.Env = []string{"IPFS_PATH=" + IpfsDirN(to)}
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("ERR: ", string(out))
		return err
	}
	addr := strings.Split(string(out), "\n")[0]

	connectcmd := exec.Command("ipfs", "swarm", "connect", addr)
	connectcmd.Env = []string{"IPFS_PATH=" + IpfsDirN(from)}
	out, err = connectcmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		return err
	}
	return nil
}

func GetAttr(attr string, node int) (string, error) {
	switch attr {
	case "id":
		return GetPeerID(node)
	default:
		return "", errors.New("unrecognized attribute")
	}
}
