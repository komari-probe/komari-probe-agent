# Komari Probe Agent

Komari Probe Agent is a lightweight monitoring component installed on monitored hosts. It collects metrics, runs probes, and reports data to the Komari Probe control plane.

English | [中文](README.md)

## Configuration

Komari Probe Agent accepts configuration through command-line flags, environment variables, or a JSON configuration file. Its stable command name is `komari-agent`.

Minimal example:

```bash
./komari-agent --endpoint "https://example.com" --token "your-token"
```

Using environment variables:

```bash
export AGENT_ENDPOINT="https://example.com"
export AGENT_TOKEN="your-token"
./komari-agent
```

Using a JSON configuration file:

```bash
./komari-agent --config ./config.json
```

Example `config.json`:

```json
{
  "endpoint": "https://example.com",
  "token": "your-token",
  "interval": 3,
  "ignore_unsafe_cert": false
}
```

Configuration precedence, from lowest to highest, is: defaults, JSON configuration file, environment variables, and explicit command-line flags. Explicit CLI flags such as `--token` always override configuration supplied by the environment or file; default flag values do not override values from the environment or JSON file.

Choose the configuration file with `--config` or `AGENT_CONFIG_FILE`. When both are supplied, `--config` takes precedence.

## Configuration reference

The supported-version column indicates the first release tag that introduced the option itself. Environment-variable and JSON configuration support is available from `1.1.33`; options predating the earliest tag are marked `0.0.9`.

| JSON field | Environment variable | CLI flag | Description | Supported since |
| --- | --- | --- | --- | --- |
| `endpoint` | `AGENT_ENDPOINT` | `--endpoint`, `-e` | Komari Probe control-plane URL | `0.0.9` |
| `token` | `AGENT_TOKEN` | `--token`, `-t` | Agent token | `0.0.9` |
| `config_file` | `AGENT_CONFIG_FILE` | `--config` | JSON configuration file path; used only to select the file to load | `1.1.33` |
| `interval` | `AGENT_INTERVAL` | `--interval`, `-i` | Metric collection interval in seconds | `0.0.9` |
| `ignore_unsafe_cert` | `AGENT_IGNORE_UNSAFE_CERT` | `--ignore-unsafe-cert`, `-u` | Skip unsafe TLS certificate verification | `0.0.9` |
| `include_nics` | `AGENT_INCLUDE_NICS` | `--include-nics` | Only collect the listed network interfaces, separated by commas | `0.0.22` |
| `exclude_nics` | `AGENT_EXCLUDE_NICS` | `--exclude-nics` | Exclude the listed network interfaces, separated by commas | `0.0.22` |
| `include_mountpoints` | `AGENT_INCLUDE_MOUNTPOINTS` | `--include-mountpoint` | Only collect the listed mount points, separated by semicolons | `0.1.0` |
| `month_rotate` | `AGENT_MONTH_ROTATE` | `--month-rotate` | Monthly traffic reset day; `0` disables the reset | `0.1.0` |
| `auto_discovery_key` | `AGENT_AUTO_DISCOVERY_KEY` | `--auto-discovery` | Auto-discovery key | `1.0.40` |
| `custom_dns` | `AGENT_CUSTOM_DNS` | `--custom-dns` | Custom DNS server | `1.0.80` |
| `enable_gpu` | `AGENT_ENABLE_GPU` | `--gpu` | Enable detailed GPU monitoring | `1.0.80` |
| `disable_compression` | `AGENT_DISABLE_COMPRESSION` | `--disable-compression` | Disable v2 transport compression | `1.2.10` |
| `prefer_ip_version` | `AGENT_PREFER_IP_VERSION` | `--prefer-ip-version` | Preferred IP version: `4` or `6` | Unreleased |
| `log_level` | `AGENT_LOG_LEVEL` | `--log-level` | Minimum log level: `debug`, `info`, `warn`, or `error`; default: `info` | Unreleased |

For all flags, run:

```bash
./komari-agent --help
```

Implementation details are in `cmd/root.go` and `internal/config/`.
