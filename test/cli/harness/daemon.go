package harness

import (
	"bytes"
	"io/ioutil"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/multiformats/go-multiaddr"
)

type Daemon struct {
	Runner  *Runner
	IPFSBin string
	APIFile string

	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer

	started bool
	mut     sync.Mutex
}

func (d *Daemon) Start(args ...string) {
	d.mut.Lock()
	defer d.mut.Unlock()
	if d.started {
		return
	}

	res := d.Runner.MustRun(RunRequest{
		Path:    d.IPFSBin,
		Args:    append([]string{"daemon"}, args...),
		RunFunc: (*exec.Cmd).Start,
	})
	d.cmd = res.Cmd
	d.stdout = res.Stdout
	d.stderr = res.Stderr

	err := WaitForFile(d.APIFile, 5*time.Second)
	if err != nil {
		log.Panicf("waiting for daemon to start (did you init?): %s", err)
	}

	d.started = true
}

func (d *Daemon) Stop() {
	d.mut.Lock()
	defer d.mut.Unlock()
	if !d.started {
		return
	}
	err := d.cmd.Process.Kill()
	if err != nil {
		log.Panicf("killing daemon: %s", err)
	}
	d.started = false
}

type API struct {
	Multiaddr string
	IPv4Addr  string
	Port      int
}

func (d *Daemon) API() API {
	d.mut.Lock()
	defer d.mut.Unlock()
	if !d.started {
		log.Panic("cannot get API info of unstarted daemon")
	}
	multiaddrBytes, err := ioutil.ReadFile(d.APIFile)
	if err != nil {
		log.Panicf("reading api file '%s': %s", d.APIFile, err)
	}
	ma, err := multiaddr.NewMultiaddr(string(multiaddrBytes))
	ipv4Addr, err := ma.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		log.Panicf("getting ipv4 address from %s: %s", ma, err)
	}
	portStr, err := ma.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		log.Panicf("getting TCP port from %s: %s", ma, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Panicf("converting port '%s' to int: %s", portStr, err)
	}

	return API{
		Multiaddr: ma.String(),
		IPv4Addr:  ipv4Addr,
		Port:      port,
	}
}

type Gateway struct {
	Multiaddr string
	IPv4Addr  string
	Port      int
}

var gatewayServerLineRegexp = regexp.MustCompile(`Gateway \(.*\) server listening on (.*)\s`)

func (d *Daemon) Gateway() Gateway {
	d.mut.Lock()
	defer d.mut.Unlock()
	if !d.started {
		log.Panic("cannot get gateway info of unstarted daemon")
	}

	matches := gatewayServerLineRegexp.FindSubmatch(d.stdout.Bytes())
	if len(matches) != 2 {
		log.Panic("can't find gateway address from daemon output")
	}
	gatewayAddr := string(matches[1])

	ma, err := multiaddr.NewMultiaddr(gatewayAddr)
	ipv4Addr, err := ma.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		log.Panicf("getting ipv4 address from %s: %s", ma, err)
	}
	portStr, err := ma.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		log.Panicf("getting TCP port from %s: %s", ma, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Panicf("converting port '%s' to int: %s", portStr, err)
	}

	return Gateway{
		Multiaddr: ma.String(),
		IPv4Addr:  ipv4Addr,
		Port:      port,
	}
}
