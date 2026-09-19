package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/pkg/idna"
)

// AutoDiscoveryConfig 自动发现配置结构体
type AutoDiscoveryConfig struct {
	UUID  string `json:"uuid"`
	Token string `json:"token"`
}

// RegisterRequest 注册请求结构体
type RegisterRequest struct {
	Key string `json:"key"`
}

// RegisterResponse 注册响应结构体
type RegisterResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		UUID  string `json:"uuid"`
		Token string `json:"token"`
	} `json:"data"`
}

// getAutoDiscoveryFilePath 获取自动发现配置文件路径
func getAutoDiscoveryFilePath() string {
	// 获取程序运行目录
	execPath, err := os.Executable()
	if err != nil {
		log.Println("Failed to get executable path:", err)
		return "auto-discovery.json"
	}
	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, "auto-discovery.json")
}

// loadAutoDiscoveryConfig 加载自动发现配置
func loadAutoDiscoveryConfig() (*AutoDiscoveryConfig, error) {
	configPath := getAutoDiscoveryFilePath()

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil // 文件不存在，返回nil
	}

	// 读取文件内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read auto-discovery config: %v", err)
	}

	// 解析JSON
	var autoDiscoveryConfig AutoDiscoveryConfig
	if err := json.Unmarshal(data, &autoDiscoveryConfig); err != nil {
		return nil, fmt.Errorf("failed to parse auto-discovery config: %v", err)
	}

	return &autoDiscoveryConfig, nil
}

// saveAutoDiscoveryConfig 保存自动发现配置
func saveAutoDiscoveryConfig(autoDiscoveryConfig *AutoDiscoveryConfig) error {
	configPath := getAutoDiscoveryFilePath()

	// 序列化为JSON
	data, err := json.MarshalIndent(autoDiscoveryConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auto-discovery config: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write auto-discovery config: %v", err)
	}

	log.Printf("Auto-discovery config saved to: %s", configPath)
	return nil
}

// registerWithAutoDiscovery uses the configured discovery key to register and
// returns the credentials that should be used for the current Agent run.
func registerWithAutoDiscovery(cfg config.Config) (*AutoDiscoveryConfig, error) {
	// 构造注册请求
	requestData := RegisterRequest{
		Key: cfg.AutoDiscoveryKey,
	}

	hostname, _ := os.Hostname()

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal register request: %v", err)
	}

	// 构造请求URL
	endpoint := cfg.Endpoint
	if len(endpoint) > 0 && endpoint[len(endpoint)-1] == '/' {
		endpoint = endpoint[:len(endpoint)-1]
	}

	// 转换中文域名为 ASCII 兼容编码
	endpoint, err = idna.ConvertIDNToASCII(endpoint)
	if err != nil {
		log.Printf("Warning: Failed to convert IDN to ASCII: %v", err)
		// 继续使用原始 endpoint，可能在某些情况下仍能工作
	}

	registerURL := fmt.Sprintf("%s/api/clients/register?name=%s", endpoint, url.QueryEscape(hostname))

	// 创建HTTP请求
	req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create register request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.AutoDiscoveryKey))

	// 发送请求
	client := connectivity.GetHTTPClientWithPreference(30*time.Second, cfg.PreferIPVersion, cfg.IgnoreUnsafeCert)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send register request: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("register request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var registerResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		return nil, fmt.Errorf("failed to parse register response: %v", err)
	}

	// 检查响应状态
	if registerResp.Status != "success" {
		return nil, fmt.Errorf("register request failed: %s", registerResp.Message)
	}

	// 保存配置
	autoDiscoveryConfig := &AutoDiscoveryConfig{
		UUID:  registerResp.Data.UUID,
		Token: registerResp.Data.Token,
	}

	if err := saveAutoDiscoveryConfig(autoDiscoveryConfig); err != nil {
		return nil, fmt.Errorf("failed to save auto-discovery config: %v", err)
	}

	log.Printf("Successfully registered with auto-discovery. UUID: %s", registerResp.Data.UUID)

	return autoDiscoveryConfig, nil
}

// ResolveAutoDiscovery loads or registers auto-discovery credentials and
// returns a copy of cfg with the effective token. It never mutates shared
// process configuration.
func ResolveAutoDiscovery(cfg config.Config) (config.Config, error) {
	// 尝试加载现有配置
	autoDiscoveryConfig, err := loadAutoDiscoveryConfig()
	if err != nil {
		log.Printf("Failed to load auto-discovery config: %v", err)
		// 继续尝试注册
	}

	if autoDiscoveryConfig != nil {
		// 配置文件存在，使用现有token
		cfg.Token = autoDiscoveryConfig.Token
		log.Printf("Using existing auto-discovery token for UUID: %s", autoDiscoveryConfig.UUID)
		return cfg, nil
	}

	// 配置文件不存在，进行注册
	log.Println("Auto-discovery config not found, registering with server...")
	autoDiscoveryConfig, err = registerWithAutoDiscovery(cfg)
	if err != nil {
		return cfg, err
	}
	cfg.Token = autoDiscoveryConfig.Token
	return cfg, nil
}
