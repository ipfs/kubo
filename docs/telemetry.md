# Telemetry Plugin Documentation

The **Telemetry plugin** is a feature in Kubo that collects **anonymized usage data** to help the development team better understand how the software is used, identify areas for improvement, and guide future feature development.

This data is not personally identifiable and is used solely for the purpose of improving the Kubo project.

---

## 🛡️ How to Control Telemetry

The behavior of the Telemetry plugin is controlled via the environment variable [`IPFS_TELEMETRY`](environment-variables.md#ipfs_telemetry) and optionally via the `Plugins.Plugins.telemetry.Config.Mode` in the IPFS config file.

### Available Modes

| Mode     | Description                                                                 |
|----------|-----------------------------------------------------------------------------|
| `on`     | **Default**. Telemetry is enabled. Data is sent periodically.              |
| `off`    | Telemetry is disabled. No data is sent. Any existing telemetry UUID file is removed. |
| `auto`   | Like `on`, but logs an informative message about the telemetry and gives user 15 minutes to opt-out before first collection. This mode is automatically used on the first run when `IPFS_TELEMETRY` is not set and telemetry UUID is not found (not generated yet). The informative message is only shown once. |

You can set the mode in your environment:

```bash
export IPFS_TELEMETRY="off"
```

Or in your IPFS config file:

```json
{
  "Plugins": {
    "Plugins": {
      "telemetry": {
        "Config": {
          "Mode": "off"
        }
      }
    }
  }
}
```

---

## 📦 What Data is Collected?

The telemetry plugin collects the following anonymized data:

### General Information

- **UUID**: Anonymous identifier for this node
- **Agent version**: Kubo version string
- **Private network**: Whether running in a private IPFS network
- **Repository size**: Categorized into privacy-preserving buckets (1GB, 5GB, 10GB, 100GB, 500GB, 1TB, 10TB, >10TB)
- **Uptime**: Categorized into privacy-preserving buckets (1d, 2d, 3d, 7d, 14d, 30d, >30d)

### Routing & Discovery

- **Custom bootstrap peers**: Whether custom `Bootstrap` peers are configured
- **Routing type**: The `Routing.Type` configured for the node
- **Accelerated DHT client**: Whether `Routing.AcceleratedDHTClient` is enabled
- **Delegated routing count**: Number of `Routing.DelegatedRouters` configured
- **AutoConf enabled**: Whether `AutoConf.Enabled` is set
- **Custom AutoConf URL**: Whether custom `AutoConf.URL` is configured
- **mDNS**: Whether `Discovery.MDNS.Enabled` is set

### Content Providing

- **Provide and Reprovide strategy**: The `Provide.Strategy` configured
- **Sweep-based provider**: Whether `Provide.DHT.SweepEnabled` is set
- **Custom Interval**: Whether custom `Provide.DHT.Interval` is configured
- **Custom MaxWorkers**: Whether custom `Provide.DHT.MaxWorkers` is configured

### Network Configuration

- **AutoNAT service mode**: The `AutoNAT.ServiceMode` configured
- **AutoNAT reachability**: Current reachability status determined by AutoNAT
- **Hole punching**: Whether `Swarm.EnableHolePunching` is enabled
- **Circuit relay addresses**: Whether the node advertises circuit relay addresses
- **Public IPv4 addresses**: Whether the node has public IPv4 addresses
- **Public IPv6 addresses**: Whether the node has public IPv6 addresses
- **AutoWSS**: Whether `AutoTLS.AutoWSS` is enabled
- **Custom domain suffix**: Whether custom `AutoTLS.DomainSuffix` is configured

### Platform Information

- **Operating system**: The OS the node is running on
- **CPU architecture**: The architecture the node is running on
- **Container detection**: Whether the node is running inside a container
- **VM detection**: Whether the node is running inside a virtual machine

### Code Reference

Data is organized in the `LogEvent` struct at [`plugin/plugins/telemetry/telemetry.go`](https://github.com/ipfs/kubo/blob/master/plugin/plugins/telemetry/telemetry.go). This struct is the authoritative source of truth for all telemetry data, including privacy-preserving buckets for repository size and uptime. Note that this documentation may not always be up-to-date - refer to the code for the current implementation.

---

## 🧑‍🤝‍🧑 Privacy and Anonymization

All data collected is:
- **Anonymized**: No personally identifiable information (PII) is sent.
- **Optional**: Users can choose to opt out at any time.
- **Secure**: Data is sent over HTTPS to a trusted endpoint.

The telemetry UUID is stored in the IPFS repo folder and is used to identify the node across runs, but it does not contain any personal information. When you opt-out, this UUID file is automatically removed to ensure complete privacy.

---

## 📦 Contributing to the Project

By enabling telemetry, you are helping the Kubo team improve the software for the entire community. The data is used to:

- Prioritize feature development
- Identify performance bottlenecks
- Improve user experience

You can always disable telemetry at any time if you change your mind.

---

## 🧪 Testing Telemetry

If you're testing telemetry locally, you can change the endpoint by setting the `Endpoint` field in the config:

```json
{
  "Plugins": {
    "Plugins": {
      "telemetry": {
        "Config": {
          "Mode": "on",
          "Endpoint": "http://localhost:8080"
        }
      }
    }
  }
}
```

This allows you to capture and inspect telemetry data locally.

---

## 📦 Further Reading

For more information, see:
- [IPFS Environment Variables](docs/environment-variables.md)
- [IPFS Plugins](docs/plugins.md)
- [IPFS Configuration](docs/config.md)
