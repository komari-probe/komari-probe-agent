# Sonar Agent

Sonar Agent is a lightweight monitoring component deployed on client servers to collect system metrics, execute ping probes, and report telemetry back to the Sonar central server.

[English](README.en.md) | [中文](README.md)

> 💡 **Tip**: To deploy the central server and web dashboard, see the [komari-probe server repository](https://github.com/sonar-probe/sonar).

---

## Installation

> 💡 **Easiest Method (Recommended)**: Log in to the Sonar Admin Panel (`/admin`), navigate to **Node Management** $\rightarrow$ click **Add Node**, and simply **copy the auto-generated one-click command** from the popup. The system automatically populates your panel address and unique token — just paste and run on your target machine!

If installing silently via CI/CD or automated scripts without using the web dashboard, use the standard commands below:

### 1. Host Installation
Compatible with Linux and macOS. Installs into `/opt/komari` by default and registers as a system service `sonar-agent.service`.

- **Stable Release**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/install.sh | sudo bash -s -- \
    -e "http://<SERVER_IP>:25774" \
    -t "<YOUR_AGENT_TOKEN>"
  ```
- **Preview / Beta Release (e.g., `v1.0.0-beta.1`)**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/install.sh | sudo bash -s -- \
    -e "http://<SERVER_IP>:25774" \
    -t "<YOUR_AGENT_TOKEN>" \
    -v "v1.0.0-beta.1"
  ```
- **Snapshot Release**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/install.sh | sudo bash -s -- \
    -e "http://<SERVER_IP>:25774" \
    -t "<YOUR_AGENT_TOKEN>" \
    --snapshot
  ```

### 2. Docker Container
> [!IMPORTANT]
> The Agent container **must run with `--net=host`**, otherwise metrics will be collected from Docker's virtual bridge rather than the physical host.

- **Stable**:
  ```bash
  docker run -d \
    --name sonar-agent \
    --restart unless-stopped \
    --net=host \
    ghcr.io/sonar-probe/sonar-agent:latest \
    -e "http://<SERVER_IP>:25774" -t "<YOUR_AGENT_TOKEN>"
  ```
- **Preview / Beta**: Change tag to `:v1.0.0-beta.1`.
- **Snapshot**: Change tag to `:snapshot`.

---

## Migration from Komari Monitor Agent

If you already have upstream Komari Monitor Agent running, automated migration scripts **preserve existing `auto-discovery.json` and active tokens**:

### 1. Host Agent Migration:
```bash
curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/scripts/migrate-agent-host.sh -o migrate-agent-host.sh
sudo bash migrate-agent-host.sh --tag v1.0.0-beta.1
```

### 2. Docker Agent Migration:
```bash
curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/scripts/migrate-agent-docker.sh -o migrate-agent-docker.sh
sudo bash migrate-agent-docker.sh \
  --container sonar-agent \
  --target-image ghcr.io/sonar-probe/sonar-agent:v1.0.0-beta.1
```

---

## Configuration & CLI Reference

Agent configuration parameters can be passed via command-line flags, environment variables, or a JSON configuration file. The binary is named `sonar-agent`.

### Quick Start Examples

1. **Direct CLI flags**:
   ```bash
   ./sonar-agent --endpoint "https://example.com" --token "your-token"
   ```
2. **Environment variables**:
   ```bash
   export AGENT_ENDPOINT="https://example.com"
   export AGENT_TOKEN="your-token"
   ./sonar-agent
   ```
3. **JSON configuration file**:
   ```bash
   ./sonar-agent --config ./config.json
   ```

`config.json` Example:
```json
{
  "endpoint": "https://example.com",
  "token": "your-token",
  "interval": 3,
  "ignore_unsafe_cert": false
}
```

Configuration precedence (lowest to highest): **Defaults < JSON Config < Environment Variables < Explicit CLI Flags**.

### Parameter Reference

| JSON Field | Environment Variable | CLI Flag | Description | Supported Since |
| :--- | :--- | :--- | :--- | :--- |
| `endpoint` | `AGENT_ENDPOINT` | `--endpoint`, `-e` | Central server endpoint URL | `0.0.9` |
| `token` | `AGENT_TOKEN` | `--token`, `-t` | Agent authentication token | `0.0.9` |
| `config_file` | `AGENT_CONFIG_FILE` | `--config` | JSON config file path | `1.1.33` |
| `interval` | `AGENT_INTERVAL` | `--interval`, `-i` | Telemetry collection interval in seconds | `0.0.9` |
| `ignore_unsafe_cert` | `AGENT_IGNORE_UNSAFE_CERT` | `--ignore-unsafe-cert`, `-u` | Skip TLS certificate verification | `0.0.9` |
| `include_nics` | `AGENT_INCLUDE_NICS` | `--include-nics` | Whitelist specific NICs (comma-separated) | `0.0.22` |
| `exclude_nics` | `AGENT_EXCLUDE_NICS` | `--exclude-nics` | Blacklist specific NICs (comma-separated) | `0.0.22` |
| `include_mountpoints`| `AGENT_INCLUDE_MOUNTPOINTS`| `--include-mountpoint` | Include specific mount points (semicolon-separated) | `0.1.0` |
| `month_rotate` | `AGENT_MONTH_ROTATE` | `--month-rotate` | Traffic monthly reset day (`0` disables) | `0.1.0` |
| `auto_discovery_key`| `AGENT_AUTO_DISCOVERY_KEY` | `--auto-discovery` | Auto-discovery registration key | `1.0.40` |
| `custom_dns` | `AGENT_CUSTOM_DNS` | `--custom-dns` | Custom DNS resolver address | `1.0.80` |
| `enable_gpu` | `AGENT_ENABLE_GPU` | `--gpu` | Enable detailed GPU monitoring | `1.0.80` |
| `disable_compression`| `AGENT_DISABLE_COMPRESSION`| `--disable-compression` | Disable payload compression | `1.2.10` |
| `prefer_ip_version` | `AGENT_PREFER_IP_VERSION` | `--prefer-ip-version` | Preferred IP family (`4` or `6`) | `v1.0.0` |
| `log_level` | `AGENT_LOG_LEVEL` | `--log-level` | Minimum log level (`debug`/`info`/`warn`/`error`) | `v1.0.0` |

View all available options:
```bash
./sonar-agent --help
```

---

## In-Place Updates

On hosts running Sonar Agent:
```bash
sonar-agent update
```
This checks the latest GitHub Release, downloads the platform-specific binary, verifies its SHA-256 against `checksums.txt`, and safely replaces the executable.
