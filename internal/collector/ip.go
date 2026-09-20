package collector

import (
	log "github.com/komari-probe/komari-probe-agent/internal/logging"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

var (
	userAgent   = "curl/8.0.1"
	ipv4Pattern = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	ipv6Pattern = regexp.MustCompile(`(([0-9A-Fa-f]{1,4}:){7})([0-9A-Fa-f]{1,4})|(([0-9A-Fa-f]{1,4}:){1,6}:)(([0-9A-Fa-f]{1,4}:){0,4})([0-9A-Fa-f]{0,4})`)
)

func (c *Collector) publicIPv4Address() string {

	webAPIs := []string{
		"https://www.visa.cn/cdn-cgi/trace",
		"https://www.qualcomm.cn/cdn-cgi/trace",
		"https://www.toutiao.com/stream/widget/local_weather/data/",
		"https://edge-ip.html.zone/geo",
		"https://vercel-ip.html.zone/geo",
		"http://ipv4.ip.sb",
		"https://api.ipify.org?format=json",
	}

	for _, api := range webAPIs {
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := c.connections.NewHTTPClientWithPreference(15*time.Second, "4", c.options.IgnoreUnsafeCert).Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // 获取后立即关闭防止堵塞
		if err != nil {
			continue
		}
		ipv4 := ipv4Pattern.FindString(string(body))
		if ipv4 != "" {
			log.Printf("Found public IPv4 address: %s", ipv4)
			return ipv4
		}
	}
	return ""
}

func (c *Collector) publicIPv6Address() string {

	webAPIs := []string{
		"https://v6.ip.zxinc.org/info.php?type=json",
		"https://api6.ipify.org?format=json",
		"https://ipv6.icanhazip.com",
		"http://api-ipv6.ip.sb/geoip",
	}

	for _, api := range webAPIs {
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := c.connections.NewHTTPClientWithPreference(15*time.Second, "6", c.options.IgnoreUnsafeCert).Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // 获取后立即关闭防止堵塞
		if err != nil {
			continue
		}

		ipv6 := ipv6Pattern.FindString(string(body))
		if ipv6 != "" {
			log.Printf("Found public IPv6 address: %s", ipv6)
			return ipv6
		}
	}
	return ""
}

func (c *Collector) IPAddresses() (ipv4, ipv6 string) {

	if c.options.GetIPAddressFromNIC {
		allowedNICs, err := c.InterfaceList()
		if err != nil {
			log.Printf("Get Interface List Error: %v", err)
		} else {
			ipv4, ipv6 = getIPFromInterfaces(allowedNICs)
			if ipv4 != "" || ipv6 != "" {
				log.Printf("Get IP from NIC - IPv4: %s, IPv6: %s", ipv4, ipv6)
				return ipv4, ipv6
			}
		}
	}

	if c.options.CustomIPv4 != "" {
		ipv4 = c.options.CustomIPv4
	} else {
		ipv4 = c.publicIPv4Address()
	}
	if c.options.CustomIPv6 != "" {
		ipv6 = c.options.CustomIPv6
	} else {
		ipv6 = c.publicIPv6Address()
	}

	return ipv4, ipv6
}

// getIPFromInterfaces 从指定的网卡接口获取 IPv4 和 IPv6 地址
func getIPFromInterfaces(nicNames []string) (ipv4, ipv6 string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get network interfaces: %v", err)
		return "", ""
	}
	for _, iface := range interfaces {
		// 检查接口是否在允许列表中
		if !containsString(nicNames, iface.Name) {
			continue
		}

		// 跳过未启动的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 获取 IPv4 地址
			if ipv4 == "" && ip.To4() != nil {
				ipv4 = ip.String()
			}

			// 获取 IPv6 地址（排除链路本地地址）
			if ipv6 == "" && ip.To4() == nil && !ip.IsLinkLocalUnicast() {
				ipv6 = ip.String()
			}

			// 如果已经找到 IPv4 和 IPv6,提前返回
			if ipv4 != "" && ipv6 != "" {
				return ipv4, ipv6
			}
		}
	}

	return ipv4, ipv6
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
