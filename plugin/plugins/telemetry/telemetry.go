package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	logging "github.com/ipfs/go-log/v2"
	ipfs "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/corerepo"
	"github.com/ipfs/kubo/plugin"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/pnet"
	multiaddr "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var log = logging.Logger("telemetry")

// Caching for virtualization detection - these values never change during process lifetime
var (
	containerDetectionOnce sync.Once
	vmDetectionOnce        sync.Once
	isContainerCached      bool
	isVMCached             bool
)

const (
	modeEnvVar   = "IPFS_TELEMETRY"
	uuidFilename = "telemetry_uuid"
	endpoint     = "https://telemetry.ipshipyard.dev"
	sendDelay    = 15 * time.Minute // delay before first telemetry collection after daemon start
	sendInterval = 24 * time.Hour   // interval between telemetry collections after the first one
	httpTimeout  = 30 * time.Second // timeout for telemetry HTTP requests
)

type pluginMode int

const (
	modeAuto pluginMode = iota
	modeOn
	modeOff
)

// repoSizeBuckets defines size thresholds for categorizing repository sizes.
// Each value represents the upper limit of a bucket in bytes (except the last)
var repoSizeBuckets = []uint64{
	1 << 30,   // 1 GB
	5 << 30,   // 5 GB
	10 << 30,  // 10 GB
	100 << 30, // 100 GB
	500 << 30, // 500 GB
	1 << 40,   // 1 TB
	10 << 40,  // 10 TB
	11 << 40,  // + anything more than 10TB falls here.
}

var uptimeBuckets = []time.Duration{
	1 * 24 * time.Hour,
	2 * 24 * time.Hour,
	3 * 24 * time.Hour,
	7 * 24 * time.Hour,
	14 * 24 * time.Hour,
	30 * 24 * time.Hour,
	31 * 24 * time.Hour, // + anything more than 30 days falls here.
}

// A LogEvent is the object sent to the telemetry endpoint.
type LogEvent struct {
	UUID string `json:"uuid"`

	AgentVersion string `json:"agent_version"`

	PrivateNetwork bool `json:"private_network"`

	BootstrappersCustom bool `json:"bootstrappers_custom"`

	RepoSizeBucket uint64 `json:"repo_size_bucket"`

	UptimeBucket time.Duration `json:"uptime_bucket"`

	ReproviderStrategy string `json:"reprovider_strategy"`

	RoutingType                 string `json:"routing_type"`
	RoutingAcceleratedDHTClient bool   `json:"routing_accelerated_dht_client"`
	RoutingDelegatedCount       int    `json:"routing_delegated_count"`

	AutoNATServiceMode  string `json:"autonat_service_mode"`
	AutoNATReachability string `json:"autonat_reachability"`

	AutoConf       bool `json:"autoconf"`
	AutoConfCustom bool `json:"autoconf_custom"`

	SwarmEnableHolePunching  bool `json:"swarm_enable_hole_punching"`
	SwarmCircuitAddresses    bool `json:"swarm_circuit_addresses"`
	SwarmIPv4PublicAddresses bool `json:"swarm_ipv4_public_addresses"`
	SwarmIPv6PublicAddresses bool `json:"swarm_ipv6_public_addresses"`

	AutoTLSAutoWSS            bool `json:"auto_tls_auto_wss"`
	AutoTLSDomainSuffixCustom bool `json:"auto_tls_domain_suffix_custom"`

	DiscoveryMDNSEnabled bool `json:"discovery_mdns_enabled"`

	PlatformOS            string `json:"platform_os"`
	PlatformArch          string `json:"platform_arch"`
	PlatformContainerized bool   `json:"platform_containerized"`
	PlatformVM            bool   `json:"platform_vm"`
}

var Plugins = []plugin.Plugin{
	&telemetryPlugin{},
}

type telemetryPlugin struct {
	uuidFilename string
	mode         pluginMode
	endpoint     string
	runOnce      bool // test-only flag: when true, sends telemetry immediately without delay
	sendDelay    time.Duration

	node      *core.IpfsNode
	config    *config.Config
	event     *LogEvent
	startTime time.Time
}

func (p *telemetryPlugin) Name() string {
	return "telemetry"
}

func (p *telemetryPlugin) Version() string {
	return "0.0.1"
}

func readFromConfig(cfg interface{}, key string) string {
	if cfg == nil {
		return ""
	}

	pcfg, ok := cfg.(map[string]interface{})
	if !ok {
		return ""
	}

	val, ok := pcfg[key].(string)
	if !ok {
		return ""
	}
	return val
}

func (p *telemetryPlugin) Init(env *plugin.Environment) error {
	// logging.SetLogLevel("telemetry", "DEBUG")
	log.Debug("telemetry plugin Init()")
	p.event = &LogEvent{}
	p.startTime = time.Now()

	repoPath := env.Repo
	p.uuidFilename = path.Join(repoPath, uuidFilename)

	v := os.Getenv(modeEnvVar)
	if v != "" {
		log.Debug("mode set from env-var")
	} else if pmode := readFromConfig(env.Config, "Mode"); pmode != "" {
		v = pmode
		log.Debug("mode set from config")
	}

	// read "Delay" from the config. Parse as duration. Set p.sendDelay to it
	// or set default.
	if delayStr := readFromConfig(env.Config, "Delay"); delayStr != "" {
		delay, err := time.ParseDuration(delayStr)
		if err != nil {
			log.Debug("sendDelay set from default")
			p.sendDelay = sendDelay
		} else {
			log.Debug("sendDelay set from config")
			p.sendDelay = delay
		}
	} else {
		log.Debug("sendDelay set from default")
		p.sendDelay = sendDelay
	}

	p.endpoint = endpoint
	if ep := readFromConfig(env.Config, "Endpoint"); ep != "" {
		log.Debug("endpoint set from config", ep)
		p.endpoint = ep
	}

	switch v {
	case "off":
		p.mode = modeOff
		log.Debug("telemetry disabled via opt-out")
		// Remove UUID file if it exists when user opts out
		if _, err := os.Stat(p.uuidFilename); err == nil {
			if err := os.Remove(p.uuidFilename); err != nil {
				log.Debugf("failed to remove telemetry UUID file: %s", err)
			} else {
				log.Debug("removed existing telemetry UUID file due to opt-out")
			}
		}
		return nil
	case "auto":
		p.mode = modeAuto
	default:
		p.mode = modeOn
	}
	log.Debug("telemetry mode: ", p.mode)
	return nil
}

func (p *telemetryPlugin) loadUUID() error {
	// Generate or read our UUID from disk
	b, err := os.ReadFile(p.uuidFilename)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("error reading telemetry uuid from disk: %s", err)
			return err
		}
		uid, err := uuid.NewRandom()
		if err != nil {
			log.Errorf("cannot generate telemetry uuid: %s", err)
			return err
		}
		p.event.UUID = uid.String()
		p.mode = modeAuto
		log.Debugf("new telemetry UUID %s. Mode set to Auto", uid)

		// Write the UUID to disk
		if err := os.WriteFile(p.uuidFilename, []byte(p.event.UUID), 0600); err != nil {
			log.Errorf("cannot write telemetry uuid: %s", err)
			return err
		}
		return nil
	}

	v := string(b)
	v = strings.TrimSpace(v)
	uid, err := uuid.Parse(v)
	if err != nil {
		log.Errorf("cannot parse telemetry uuid: %s", err)
		return err
	}
	log.Debugf("uuid read from disk %s", uid)
	p.event.UUID = uid.String()
	return nil
}

func (p *telemetryPlugin) hasDefaultBootstrapPeers() bool {
	// With autoconf, default bootstrap is represented as ["auto"]
	currentPeers := p.config.Bootstrap
	return len(currentPeers) == 1 && currentPeers[0] == "auto"
}

func (p *telemetryPlugin) showInfo() {
	fmt.Printf(`

ℹ️  Anonymous telemetry will be enabled in %s

Kubo will collect anonymous usage data to help improve the software:
• What:  Feature usage and configuration (no personal data)
         Use GOLOG_LOG_LEVEL="telemetry=debug" to inspect collected data
• When:  First collection in %s, then every 24h
• How:   HTTP POST to %s
         Anonymous ID: %s

No data sent yet. To opt-out before collection starts:
• Set environment: %s=off
• Or run: ipfs config Plugins.Plugins.telemetry.Config.Mode off
• Then restart daemon

This message is shown only once.
Learn more: https://github.com/ipfs/kubo/blob/master/docs/telemetry.md


`, p.sendDelay, p.sendDelay, endpoint, p.event.UUID, modeEnvVar)
}

// Start finishes telemetry initialization once the IpfsNode is ready,
// collects telemetry data and sends it to the endpoint.
func (p *telemetryPlugin) Start(n *core.IpfsNode) error {
	// We should not be crashing the daemon due to problems with telemetry
	// so this is always going to return nil and panics are going to be
	// handled.
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("telemetry plugin panicked: %v", r)
		}
	}()

	p.node = n
	cfg, err := n.Repo.Config()
	if err != nil {
		log.Error("error getting the repo.Config: %s", err)
		return nil
	}
	p.config = cfg
	if p.mode == modeOff {
		log.Debug("telemetry collection skipped: opted out")
		return nil
	}

	if !n.IsDaemon || !n.IsOnline {
		log.Debugf("skipping telemetry. Daemon: %t. Online: %t", n.IsDaemon, n.IsOnline)
		return nil
	}

	// loadUUID might switch to modeAuto when generating a new uuid
	if err := p.loadUUID(); err != nil {
		p.mode = modeOff
		return nil
	}

	if p.mode == modeAuto {
		p.showInfo()
	}

	// runOnce is only used in tests to send telemetry immediately.
	// In production, this is always false, ensuring users get the 15-minute delay.
	if p.runOnce {
		p.prepareEvent()
		return p.sendTelemetry()
	}

	go func() {
		timer := time.NewTimer(p.sendDelay)
		for range timer.C {
			p.prepareEvent()
			if err := p.sendTelemetry(); err != nil {
				log.Warnf("telemetry submission failed: %s (will retry in %s)", err, sendInterval)
			}
			timer.Reset(sendInterval)
		}
	}()

	return nil
}

func (p *telemetryPlugin) prepareEvent() {
	p.collectBasicInfo()
	p.collectRoutingInfo()
	p.collectAutoNATInfo()
	p.collectAutoConfInfo()
	p.collectSwarmInfo()
	p.collectAutoTLSInfo()
	p.collectDiscoveryInfo()
	p.collectPlatformInfo()
}

// Collects:
// * AgentVersion
// * PrivateNetwork
// * RepoSizeBucket
// * BootstrappersCustom
// * UptimeBucket
// * ReproviderStrategy
func (p *telemetryPlugin) collectBasicInfo() {
	p.event.AgentVersion = ipfs.GetUserAgentVersion()

	privNet := false
	if pnet.ForcePrivateNetwork {
		privNet = true
	} else if key, _ := p.node.Repo.SwarmKey(); key != nil {
		privNet = true
	}
	p.event.PrivateNetwork = privNet

	p.event.BootstrappersCustom = !p.hasDefaultBootstrapPeers()

	repoSizeBucket := repoSizeBuckets[len(repoSizeBuckets)-1]
	sizeStat, err := corerepo.RepoSize(context.Background(), p.node)
	if err == nil {
		for _, b := range repoSizeBuckets {
			if sizeStat.RepoSize > b {
				continue
			}
			repoSizeBucket = b
			break
		}
		p.event.RepoSizeBucket = repoSizeBucket
	} else {
		log.Debugf("error setting sizeStat: %s", err)
	}

	uptime := time.Since(p.startTime)
	uptimeBucket := uptimeBuckets[len(uptimeBuckets)-1]
	for _, bucket := range uptimeBuckets {
		if uptime > bucket {
			continue

		}
		uptimeBucket = bucket
		break
	}
	p.event.UptimeBucket = uptimeBucket

	p.event.ReproviderStrategy = p.config.Provide.Strategy.WithDefault(config.DefaultProvideStrategy)
}

func (p *telemetryPlugin) collectRoutingInfo() {
	p.event.RoutingType = p.config.Routing.Type.WithDefault("auto")
	p.event.RoutingAcceleratedDHTClient = p.config.Routing.AcceleratedDHTClient.WithDefault(false)
	p.event.RoutingDelegatedCount = len(p.config.Routing.DelegatedRouters)
}

type reachabilityHost interface {
	Reachability() network.Reachability
}

func (p *telemetryPlugin) collectAutoNATInfo() {
	autonat := p.config.AutoNAT.ServiceMode
	if autonat == config.AutoNATServiceUnset {
		autonat = config.AutoNATServiceEnabled
	}
	autoNATSvcModeB, err := autonat.MarshalText()
	if err == nil {
		autoNATSvcMode := string(autoNATSvcModeB)
		if autoNATSvcMode == "" {
			autoNATSvcMode = "unset"
		}
		p.event.AutoNATServiceMode = autoNATSvcMode
	}

	h := p.node.PeerHost
	reachHost, ok := h.(reachabilityHost)
	if ok {
		p.event.AutoNATReachability = reachHost.Reachability().String()
	}
}

func (p *telemetryPlugin) collectSwarmInfo() {
	p.event.SwarmEnableHolePunching = p.config.Swarm.EnableHolePunching.WithDefault(true)

	var circuitAddrs, publicIP4Addrs, publicIP6Addrs bool
	for _, addr := range p.node.PeerHost.Addrs() {
		if manet.IsPublicAddr(addr) {
			if _, err := addr.ValueForProtocol(multiaddr.P_IP4); err == nil {
				publicIP4Addrs = true
			} else if _, err := addr.ValueForProtocol(multiaddr.P_IP6); err == nil {
				publicIP6Addrs = true
			}
		}
		if _, err := addr.ValueForProtocol(multiaddr.P_CIRCUIT); err == nil {
			circuitAddrs = true
		}
	}

	p.event.SwarmCircuitAddresses = circuitAddrs
	p.event.SwarmIPv4PublicAddresses = publicIP4Addrs
	p.event.SwarmIPv6PublicAddresses = publicIP6Addrs
}

func (p *telemetryPlugin) collectAutoTLSInfo() {
	p.event.AutoTLSAutoWSS = p.config.AutoTLS.AutoWSS.WithDefault(config.DefaultAutoWSS)
	domainSuffix := p.config.AutoTLS.DomainSuffix.WithDefault(config.DefaultDomainSuffix)
	p.event.AutoTLSDomainSuffixCustom = domainSuffix != config.DefaultDomainSuffix
}

func (p *telemetryPlugin) collectAutoConfInfo() {
	p.event.AutoConf = p.config.AutoConf.Enabled.WithDefault(config.DefaultAutoConfEnabled)
	p.event.AutoConfCustom = p.config.AutoConf.URL.WithDefault(config.DefaultAutoConfURL) != config.DefaultAutoConfURL
}

func (p *telemetryPlugin) collectDiscoveryInfo() {
	p.event.DiscoveryMDNSEnabled = p.config.Discovery.MDNS.Enabled
}

func (p *telemetryPlugin) collectPlatformInfo() {
	p.event.PlatformOS = runtime.GOOS
	p.event.PlatformArch = runtime.GOARCH
	p.event.PlatformContainerized = isRunningInContainer()
	p.event.PlatformVM = isRunningInVM()
}

func isRunningInContainer() bool {
	containerDetectionOnce.Do(func() {
		isContainerCached = detectContainer()
	})
	return isContainerCached
}

func detectContainer() bool {
	// Docker creates /.dockerenv inside containers
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Kubernetes mounts service account tokens inside pods
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
		return true
	}

	// systemd-nspawn creates this file inside containers
	if _, err := os.Stat("/run/systemd/container"); err == nil {
		return true
	}

	// Check if our process is running inside a container cgroup
	// Look for container-specific patterns in the cgroup path after "::/"
	if content, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		for line := range strings.Lines(string(content)) {
			// cgroup lines format: "ID:subsystem:/path"
			// We want to check the path part after the last ":"
			parts := strings.SplitN(line, ":", 3)
			if len(parts) == 3 {
				cgroupPath := parts[2]
				// Check for container-specific paths
				containerIndicators := []string{
					"/docker/",     // Docker containers
					"/containerd/", // containerd runtime
					"/cri-o/",      // CRI-O runtime
					"/lxc/",        // LXC containers
					"/podman/",     // Podman containers
					"/kubepods/",   // Kubernetes pods
				}
				for _, indicator := range containerIndicators {
					if strings.Contains(cgroupPath, indicator) {
						return true
					}
				}
			}
		}
	}

	// WSL is technically a container-like environment
	if runtime.GOOS == "linux" {
		if content, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			osrelease := strings.ToLower(string(content))
			if strings.Contains(osrelease, "microsoft") || strings.Contains(osrelease, "wsl") {
				return true
			}
		}
	}

	// LXC sets container environment variable
	if content, err := os.ReadFile("/proc/1/environ"); err == nil {
		if strings.Contains(string(content), "container=lxc") {
			return true
		}
	}

	// Additional check: In containers, PID 1 is often not systemd/init
	if content, err := os.ReadFile("/proc/1/comm"); err == nil {
		pid1 := strings.TrimSpace(string(content))
		// Common container init processes
		containerInits := []string{"tini", "dumb-init", "s6-svscan", "runit"}
		if slices.Contains(containerInits, pid1) {
			return true
		}
	}

	return false
}

func isRunningInVM() bool {
	vmDetectionOnce.Do(func() {
		isVMCached = detectVM()
	})
	return isVMCached
}

func detectVM() bool {
	// Check for VM-specific files and drivers that only exist inside VMs
	vmIndicators := []string{
		"/proc/xen",               // Xen hypervisor guest
		"/sys/hypervisor/uuid",    // KVM/Xen hypervisor guest
		"/dev/vboxguest",          // VirtualBox guest additions
		"/sys/module/vmw_balloon", // VMware balloon driver (guest only)
		"/sys/module/hv_vmbus",    // Hyper-V VM bus driver (guest only)
	}

	for _, path := range vmIndicators {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	// Check DMI for VM vendors - these strings only appear inside VMs
	// DMI (Desktop Management Interface) is populated by the hypervisor
	dmiFiles := map[string][]string{
		"/sys/class/dmi/id/sys_vendor": {
			"qemu", "kvm", "vmware", "virtualbox", "xen",
			"parallels", // Parallels Desktop
			// Note: Removed "microsoft corporation" as it can match Surface devices
		},
		"/sys/class/dmi/id/product_name": {
			"virtualbox", "vmware", "kvm", "qemu",
			"hvm domu", // Xen HVM guest
			// Note: Removed generic "virtual machine" to avoid false positives
		},
		"/sys/class/dmi/id/chassis_vendor": {
			"qemu", "oracle", // Oracle for VirtualBox
		},
	}

	for path, signatures := range dmiFiles {
		if content, err := os.ReadFile(path); err == nil {
			contentStr := strings.ToLower(strings.TrimSpace(string(content)))
			for _, sig := range signatures {
				if strings.Contains(contentStr, sig) {
					return true
				}
			}
		}
	}

	return false
}

func (p *telemetryPlugin) sendTelemetry() error {
	data, err := json.MarshalIndent(p.event, "", "  ")
	if err != nil {
		return err
	}

	log.Debugf("sending telemetry:\n %s", data)

	req, err := http.NewRequest("POST", p.endpoint, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ipfs.GetUserAgentVersion())
	req.Close = true

	// Use client with timeout to prevent hanging
	client := &http.Client{
		Timeout: httpTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Debugf("failed to send telemetry: %s", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err := fmt.Errorf("telemetry endpoint returned HTTP %d", resp.StatusCode)
		log.Debug(err)
		return err
	}
	log.Debugf("telemetry sent successfully (%d)", resp.StatusCode)
	return nil
}
