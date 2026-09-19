package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
)

type Config struct {
	AutoDiscoveryKey    string  `json:"auto_discovery_key" env:"AGENT_AUTO_DISCOVERY_KEY"`         // 自动发现密钥
	Token               string  `json:"token" env:"AGENT_TOKEN"`                                   // Token
	Endpoint            string  `json:"endpoint" env:"AGENT_ENDPOINT"`                             // 面板地址
	Interval            float64 `json:"interval" env:"AGENT_INTERVAL"`                             // 数据采集间隔，单位秒
	IgnoreUnsafeCert    bool    `json:"ignore_unsafe_cert" env:"AGENT_IGNORE_UNSAFE_CERT"`         // 忽略不安全的证书
	MaxRetries          int     `json:"max_retries" env:"AGENT_MAX_RETRIES"`                       // 最大重试次数
	ReconnectInterval   int     `json:"reconnect_interval" env:"AGENT_RECONNECT_INTERVAL"`         // 重连间隔，单位秒
	InfoReportInterval  int     `json:"info_report_interval" env:"AGENT_INFO_REPORT_INTERVAL"`     // 基础信息上报间隔，单位分钟
	IncludeNics         string  `json:"include_nics" env:"AGENT_INCLUDE_NICS"`                     // 仅统计网卡，逗号分隔的网卡名称列表，支持通配符
	ExcludeNics         string  `json:"exclude_nics" env:"AGENT_EXCLUDE_NICS"`                     // 统计时排除的网卡，逗号分隔的网卡名称列表，支持通配符
	IncludeMountpoints  string  `json:"include_mountpoints" env:"AGENT_INCLUDE_MOUNTPOINTS"`       // 磁盘统计的包含挂载点列表，使用分号分隔
	MonthRotate         int     `json:"month_rotate" env:"AGENT_MONTH_ROTATE"`                     // 流量统计的月份重置日期（0表示禁用）
	MemoryIncludeCache  bool    `json:"memory_include_cache" env:"AGENT_MEMORY_INCLUDE_CACHE"`     // 包括缓存/缓冲区的内存使用情况
	MemoryReportRawUsed bool    `json:"memory_report_raw_used" env:"AGENT_MEMORY_REPORT_RAW_USED"` // 使用原始内存使用情况报告
	CustomDNS           string  `json:"custom_dns" env:"AGENT_CUSTOM_DNS"`                         // 使用的自定义DNS服务器
	EnableGPU           bool    `json:"enable_gpu" env:"AGENT_ENABLE_GPU"`                         // 启用详细GPU监控
	CustomIpv4          string  `json:"custom_ipv4" env:"AGENT_CUSTOM_IPV4"`                       // 自定义 IPv4 地址
	CustomIpv6          string  `json:"custom_ipv6" env:"AGENT_CUSTOM_IPV6"`                       // 自定义 IPv6 地址
	GetIpAddrFromNic    bool    `json:"get_ip_addr_from_nic" env:"AGENT_GET_IP_ADDR_FROM_NIC"`     // 从网卡获取IP地址
	HostProc            string  `json:"host_proc" env:"HOST_PROC"`                                 // 容器环境下宿主机/proc目录的挂载点，用于监控宿主机进程
	ConfigFile          string  `json:"config_file" env:"AGENT_CONFIG_FILE"`                       // JSON配置文件路径
	DisableCompression  bool    `json:"disable_compression" env:"AGENT_DISABLE_COMPRESSION"`       // 禁用v2传输压缩
	PreferIPVersion     string  `json:"prefer_ip_version" env:"AGENT_PREFER_IP_VERSION"`           // 面板连接优先使用的 IP 版本：4 或 6

}

// Default returns the built-in configuration values. Configuration is resolved
// in this order: defaults, configuration file, environment, then CLI flags.
func Default() Config {
	return Config{
		Interval:           3.0,
		MaxRetries:         3,
		ReconnectInterval:  5,
		InfoReportInterval: 5,
	}
}

var GlobalConfig = func() *Config {
	defaults := Default()
	return &defaults
}()

// LoadFile overlays the JSON configuration at path onto dst. Callers should
// initialise dst with Default before loading a file so omitted fields retain
// their defaults.
func LoadFile(path string, dst *Config) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(contents, dst); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	return nil
}

// ApplyEnvironment overlays values from environment variables declared in the
// env tag of Config fields. A set but invalid value is an error rather than
// being silently ignored.
func ApplyEnvironment(dst *Config, lookup func(string) (string, bool)) error {
	value := reflect.ValueOf(dst).Elem()
	typ := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		envName := typ.Field(i).Tag.Get("env")
		if envName == "" {
			continue
		}
		envValue, ok := lookup(envName)
		if !ok {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Bool:
			parsed, err := strconv.ParseBool(envValue)
			if err != nil {
				return fmt.Errorf("parse %s as bool: %w", envName, err)
			}
			field.SetBool(parsed)
		case reflect.Int:
			parsed, err := strconv.Atoi(envValue)
			if err != nil {
				return fmt.Errorf("parse %s as integer: %w", envName, err)
			}
			field.SetInt(int64(parsed))
		case reflect.Float64:
			parsed, err := strconv.ParseFloat(envValue, 64)
			if err != nil {
				return fmt.Errorf("parse %s as number: %w", envName, err)
			}
			field.SetFloat(parsed)
		default:
			return fmt.Errorf("unsupported config field type %s for %s", field.Kind(), envName)
		}
	}
	return nil
}
