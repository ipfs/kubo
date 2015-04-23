package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	serial "github.com/ipfs/go-ipfs/repo/fsrepo/serialize"
)

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

type initCfg struct {
	Count     int
	Force     bool
	Bootstrap string
}

func IpfsInit(cfg *initCfg) error {
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

	return nil
}

func starBootstrap(cfg *initCfg) error {
	// '0' node is the bootstrap node
	cfgpath := path.Join(IpfsDirN(0), "config")
	bcfg, err := serial.Load(cfgpath)
	if err != nil {
		return err
	}
	bcfg.Bootstrap = nil
	bcfg.Addresses.Swarm = []string{"/ip4/127.0.0.1/tcp/4002"}
	bcfg.Addresses.API = "/ip4/127.0.0.1/tcp/5002"
	bcfg.Addresses.Gateway = ""
	err = serial.WriteConfigFile(cfgpath, bcfg)
	if err != nil {
		return err
	}

	for i := 1; i < cfg.Count; i++ {
		cfgpath := path.Join(IpfsDirN(i), "config")
		cfg, err := serial.Load(cfgpath)
		if err != nil {
			return err
		}

		cfg.Bootstrap = []string{fmt.Sprintf("%s/ipfs/%s", bcfg.Addresses.Swarm[0], bcfg.Identity.PeerID)}
		cfg.Addresses.Gateway = ""
		cfg.Addresses.Swarm = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", 4002+i),
		}
		cfg.Addresses.API = fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 5002+i)
		err = serial.WriteConfigFile(cfgpath, cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func clearBootstrapping(cfg *initCfg) error {
	for i := 0; i < cfg.Count; i++ {
		cfgpath := path.Join(IpfsDirN(i), "config")
		cfg, err := serial.Load(cfgpath)
		if err != nil {
			return err
		}

		cfg.Bootstrap = nil
		cfg.Addresses.Gateway = ""
		cfg.Addresses.Swarm = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", 4002+i),
		}
		cfg.Addresses.API = fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", 5002+i)
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

func IpfsKill() error {
	n := GetNumNodes()
	for i := 0; i < n; i++ {
		pid, err := IpfsPidOf(i)
		if err != nil {
			fmt.Printf("error killing daemon %d: %s\n", i, err)
			continue
		}

		p, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("error killing daemon %d: %s\n", i, err)
			continue
		}
		err = p.Kill()
		if err != nil {
			fmt.Printf("error killing daemon %d: %s\n", i, err)
			continue
		}

		p.Wait()

		err = os.Remove(path.Join(IpfsDirN(i), "daemon.pid"))
		if err != nil {
			fmt.Printf("error removing pid file for daemon %d: %s\n", i, err)
			continue
		}
	}
	return nil
}

func IpfsStart(waitall bool) error {
	n := GetNumNodes()
	for i := 0; i < n; i++ {
		dir := IpfsDirN(i)
		cmd := exec.Command("ipfs", "daemon")
		cmd.Dir = dir
		cmd.Env = []string{"IPFS_PATH=" + dir}

		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

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
		if i == 0 || waitall {
			err := waitForLive(fmt.Sprintf("localhost:%d", 5002+i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// waitForLive polls the given endpoint until it is up, or until
// a timeout
func waitForLive(addr string) error {
	for i := 0; i < 50; i++ {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
			return nil
		}
		time.Sleep(time.Millisecond * 200)
	}
	return fmt.Errorf("node at %s failed to come online in given time period", addr)
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

func GetAttr(attr string, node int) (string, error) {
	switch attr {
	case "id":
		return GetPeerID(node)
	default:
		return "", errors.New("unrecognized attribute")
	}
}

var helptext = `Ipfs Testbed

Commands:
	init 
	    creates and initializes 'n' repos

		Options:
			-n=[number of nodes]
			-f - force overwriting of existing nodes
			-bootstrap - select bootstrapping style for cluster
				choices: star, none

	start 
	    starts up all testbed nodes

		Options:
			-wait - wait until daemons are fully initialized
	stop 
	    kills all testbed nodes
	restart
	    kills, then restarts all testbed nodes

	shell [n]
	    execs your shell with environment variables set as follows:
	        IPFS_PATH - set to testbed node n's IPFS_PATH
	        NODE[x] - set to the peer ID of node x

	get [attribute] [node]
		get an attribute of the given node
		currently supports: "id"

Env Vars:

IPTB_ROOT:
	Used to specify the directory that nodes will be created in.
`

func handleErr(s string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, s, err)
		os.Exit(1)
	}
}

func main() {
	cfg := new(initCfg)
	flag.IntVar(&cfg.Count, "n", 0, "number of ipfs nodes to initialize")
	flag.BoolVar(&cfg.Force, "f", false, "force initialization (overwrite existing configs)")
	flag.StringVar(&cfg.Bootstrap, "bootstrap", "star", "select bootstrapping style for cluster")

	wait := flag.Bool("wait", false, "wait for nodes to come fully online before exiting")
	flag.Usage = func() {
		fmt.Println(helptext)
	}

	flag.Parse()

	switch flag.Arg(0) {
	case "init":
		if cfg.Count == 0 {
			fmt.Printf("please specify number of nodes: '%s -n=10 init'\n", os.Args[0])
			os.Exit(1)
		}
		err := IpfsInit(cfg)
		handleErr("ipfs init err: ", err)
	case "start":
		err := IpfsStart(*wait)
		handleErr("ipfs start err: ", err)
	case "stop", "kill":
		err := IpfsKill()
		handleErr("ipfs kill err: ", err)
	case "restart":
		err := IpfsKill()
		handleErr("ipfs kill err: ", err)

		err = IpfsStart(*wait)
		handleErr("ipfs start err: ", err)
	case "shell":
		if len(flag.Args()) < 2 {
			fmt.Println("please specify which node you want a shell for")
			os.Exit(1)
		}
		n, err := strconv.Atoi(flag.Arg(1))
		handleErr("parse err: ", err)

		err = IpfsShell(n)
		handleErr("ipfs shell err: ", err)
	case "get":
		if len(flag.Args()) < 3 {
			fmt.Println("iptb get [attr] [node]")
			os.Exit(1)
		}
		attr := flag.Arg(1)
		num, err := strconv.Atoi(flag.Arg(2))
		handleErr("error parsing node number: ", err)

		val, err := GetAttr(attr, num)
		handleErr("error getting attribute: ", err)
		fmt.Println(val)
	default:
		flag.Usage()
		os.Exit(1)
	}
}
