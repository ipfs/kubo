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
- **Agent version**: The version of Kubo being used.
- **Platform details**: Operating system, architecture, and container status.
- **Uptime**: How long the node has been running, categorized into buckets.
- **Repo size**: Categorized into buckets (e.g., 1GB, 5GB, 10GB, etc.).

### Network Configuration
- **Private network**: Whether the node is running in a private network.
- **Bootstrap peers**: Whether custom bootstrap peers are used.
- **Routing type**: Whether the node uses DHT, IPFS, or a custom routing setup.
- **AutoNAT settings**: Whether AutoNAT is enabled and its reachability status.
- **Swarm settings**: Whether hole punching is enabled, and whether public IP addresses are used.

### TLS and Discovery
- **AutoTLS settings**: Whether WSS is enabled and whether a custom domain suffix is used.
- **Discovery settings**: Whether mDNS is enabled.

### Reprovider Strategy
- The strategy used for reprovider (e.g., "all", "pinned"...).

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
