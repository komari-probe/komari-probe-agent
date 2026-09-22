# Sonar Agent

Sonar Agent 是部署在被监控主机上的轻量级探针组件，负责采集主机指标、执行网络探测并向 Sonar 中心控制端上报数据。

[English](README.en.md) | 中文

> 💡 **提示**：如需部署中心监控控制面板，请参阅 [Sonar 服务端仓库](https://github.com/sonar-probe/sonar)。

---

## 快速安装 (Installation)

> 💡 **最简接入方式（推荐）**：登录 Sonar 服务端管理后台（`/admin`），点击 **【节点管理】** $\rightarrow$ **【添加节点】**，在弹出的窗口中**直接复制系统自动生成的一键安装命令**并在目标服务器终端粘贴运行即可！系统已自动拼装好当前服务端通信地址与专属 Token，无需手动替换任何参数。

若需在 CI/CD 或自动化部署脚本中静默安装，可参考以下标准命令：

### 1. 宿主机一键安装
适用于 Linux / macOS。默认安装至 `/opt/komari`，并注册为系统服务 `sonar-agent.service`。

- **正式稳定版（Stable）**：
  ```bash
  curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/install.sh | sudo bash -s -- \
    -e "http://<服务端IP或域名>:25774" \
    -t "<你的AGENT_TOKEN>"
  ```
- **预览体验版（Pre-release / Beta，如 `v1.0.0-beta.1`）**：
  ```bash
  curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/install.sh | sudo bash -s -- \
    -e "http://<服务端IP或域名>:25774" \
    -t "<你的AGENT_TOKEN>" \
    -v "v1.0.0-beta.1"
  ```
- **开发快照版（Snapshot）**：
  ```bash
  curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/install.sh | sudo bash -s -- \
    -e "http://<服务端IP或域名>:25774" \
    -t "<你的AGENT_TOKEN>" \
    --snapshot
  ```

### 2. Docker 方式运行
> [!IMPORTANT]
> Agent 容器**必须使用 `--net=host`**，否则采集到的将是 Docker 内部虚拟网桥数据而非物理主机的真实 CPU、内存及网络数据。

- **正式稳定版**：
  ```bash
  docker run -d \
    --name sonar-agent \
    --restart unless-stopped \
    --net=host \
    ghcr.io/sonar-probe/sonar-agent:latest \
    -e "http://<服务端IP或域名>:25774" -t "<你的AGENT_TOKEN>"
  ```
- **预览体验版**：将镜像标签改为 `:v1.0.0-beta.1`。
- **开发快照版**：将镜像标签改为 `:snapshot`。

---

## 从原版 Komari Monitor Agent 平滑迁移 (Migration)

如果你此前已运行原版 Komari Monitor 探针，迁移脚本会**自动备份旧程序，并完整保留已有的 `auto-discovery.json` 与已连接 Token**：

### 1. 宿主机方式迁移：
```bash
# 下载迁移脚本
curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/scripts/migrate-agent-host.sh -o migrate-agent-host.sh

# 执行迁移（预览期指定 --tag v1.0.0-beta.1）
sudo bash migrate-agent-host.sh --tag v1.0.0-beta.1
```

### 2. Docker 方式迁移：
```bash
# 下载 Docker 迁移脚本
curl -fsSL https://raw.githubusercontent.com/sonar-probe/sonar-agent/main/scripts/migrate-agent-docker.sh -o migrate-agent-docker.sh

# 执行迁移
sudo bash migrate-agent-docker.sh \
  --container sonar-agent \
  --target-image ghcr.io/sonar-probe/sonar-agent:v1.0.0-beta.1
```

---

## 配置方式与参数字典

Sonar Agent 参数可以通过命令行参数、环境变量或 JSON 配置文件传入。正式命令为 `sonar-agent`。

### 常用启动方式

1. **直接传参启动**：
   ```bash
   ./sonar-agent --endpoint "https://example.com" --token "your-token"
   ```
2. **使用环境变量**：
   ```bash
   export AGENT_ENDPOINT="https://example.com"
   export AGENT_TOKEN="your-token"
   ./sonar-agent
   ```
3. **使用 JSON 配置文件**：
   ```bash
   ./sonar-agent --config ./config.json
   ```

`config.json` 示例：
```json
{
  "endpoint": "https://example.com",
  "token": "your-token",
  "interval": 3,
  "ignore_unsafe_cert": false
}
```

配置优先级从低到高为：**默认值 < JSON 配置文件 < 环境变量 < 显式命令行参数**。

### 参数对照字典

| JSON 字段 | 环境变量 | 命令行参数 | 说明 | 支持版本 |
| :--- | :--- | :--- | :--- | :--- |
| `endpoint` | `AGENT_ENDPOINT` | `--endpoint`, `-e` | 中心面板地址 | `0.0.9` |
| `token` | `AGENT_TOKEN` | `--token`, `-t` | Agent 节点通信 Token | `0.0.9` |
| `config_file` | `AGENT_CONFIG_FILE` | `--config` | JSON 配置文件路径 | `1.1.33` |
| `interval` | `AGENT_INTERVAL` | `--interval`, `-i` | 数据采集上报间隔（单位秒，默认 3） | `0.0.9` |
| `ignore_unsafe_cert` | `AGENT_IGNORE_UNSAFE_CERT` | `--ignore-unsafe-cert`, `-u` | 忽略 HTTPS 证书校验 | `0.0.9` |
| `include_nics` | `AGENT_INCLUDE_NICS` | `--include-nics` | 仅统计指定网卡（逗号分隔） | `0.0.22` |
| `exclude_nics` | `AGENT_EXCLUDE_NICS` | `--exclude-nics` | 排除指定网卡（逗号分隔） | `0.0.22` |
| `include_mountpoints`| `AGENT_INCLUDE_MOUNTPOINTS`| `--include-mountpoint` | 仅统计指定挂载点（分号分隔） | `0.1.0` |
| `month_rotate` | `AGENT_MONTH_ROTATE` | `--month-rotate` | 流量统计每月重置日期（`0` 为禁用） | `0.1.0` |
| `auto_discovery_key`| `AGENT_AUTO_DISCOVERY_KEY` | `--auto-discovery` | 自动发现密钥 | `1.0.40` |
| `custom_dns` | `AGENT_CUSTOM_DNS` | `--custom-dns` | 自定义 DNS 服务器 | `1.0.80` |
| `enable_gpu` | `AGENT_ENABLE_GPU` | `--gpu` | 启用详细 GPU 监控 | `1.0.80` |
| `disable_compression`| `AGENT_DISABLE_COMPRESSION`| `--disable-compression` | 禁用传输压缩 | `1.2.10` |
| `prefer_ip_version` | `AGENT_PREFER_IP_VERSION` | `--prefer-ip-version` | 优先使用的 IP 版本（`4` 或 `6`） | `v1.0.0` |
| `log_level` | `AGENT_LOG_LEVEL` | `--log-level` | 最低日志等级（`debug` / `info` / `warn` / `error`） | `v1.0.0` |

查看完整参数说明：
```bash
./sonar-agent --help
```

---

## 手动更新

在已安装 Agent 的主机上运行：
```bash
sonar-agent update
```
该命令会检查官方 GitHub Release，按当前系统和架构下载二进制，并比对 Release 中的 `checksums.txt` 校验 SHA-256，验证无误后替换文件并提示重启服务。
