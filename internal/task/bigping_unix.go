//go:build !windows

package task

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

// BigPacketProbe 对目标发送一个填充到约 1400 字节的 TCP SYN 包，测量到收到
// SYN/ACK 或 RST 为止的往返时间。这是手动"立即测一次"专用的诊断探测，
// 做法跟 mtr/nexttrace 的 `--tcp -s 1400` 一致：不完成三次握手，收到对方
// 回应后立刻发 RST 结束，不占用目标资源、不建立真正的连接。
//
// 这不是持续后台监控用的探测方式（持续监控走 tcpPing 完整握手，见
// ping.go），BigPacketProbe 只应该由用户手动触发的一次性测试调用，
// 不要接入 PingTask 的常规探测循环。
//
// 需要 raw socket 权限（root 或 CAP_NET_RAW），跟现有 icmpPing 要求一致。
// 仅支持 Linux/macOS 等类 Unix 系统；Windows 从 XP SP2 起限制普通程序
// 通过 raw socket 发送自定义 TCP 包，要绕开就得引入 Npcap/WinPcap 之类的
// 第三方驱动，这会破坏探针"单文件、零外部依赖"的部署方式，所以 Windows
// 不提供这个功能，见 bigping_windows.go。
const bigPacketTotalSize = 1400

const (
	tcpFlagFIN = 0x01
	tcpFlagSYN = 0x02
	tcpFlagRST = 0x04
	tcpFlagACK = 0x10
)

func BigPacketProbe(ctx context.Context, target string, timeout time.Duration) (int64, error) {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		host = target
		portStr = "80"
	}
	host = strings.Trim(host, "[]")

	dstIP, err := resolveIP(ctx, host)
	if err != nil {
		return -1, err
	}
	dstIPParsed := net.ParseIP(dstIP).To4()
	if dstIPParsed == nil {
		return -1, fmt.Errorf("big packet probe currently only supports IPv4 targets, got %q", dstIP)
	}
	dstPort, err := strconv.Atoi(portStr)
	if err != nil || dstPort <= 0 || dstPort > 65535 {
		return -1, fmt.Errorf("invalid port %q", portStr)
	}

	srcIP, err := outboundIPv4(dstIPParsed)
	if err != nil {
		return -1, fmt.Errorf("determine outbound source ip: %w", err)
	}

	conn, err := net.ListenPacket("ip4:tcp", srcIP.String())
	if err != nil {
		return -1, fmt.Errorf("open raw tcp socket (requires root/CAP_NET_RAW): %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return -1, err
	}

	srcPort := uint16(20000 + rand.Intn(20000))
	seq := rand.Uint32()
	dstAddr := &net.IPAddr{IP: dstIPParsed}

	segment := buildSYNSegment(srcIP, dstIPParsed, srcPort, uint16(dstPort), seq, bigPacketTotalSize)

	start := time.Now()
	if _, err := conn.WriteTo(segment, dstAddr); err != nil {
		return -1, fmt.Errorf("send syn: %w", err)
	}

	buf := make([]byte, 65535)
	for {
		if ctx.Err() != nil {
			return -1, ctx.Err()
		}
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			return -1, fmt.Errorf("no response before timeout: %w", err)
		}
		peerIP, ok := peer.(*net.IPAddr)
		if !ok || !peerIP.IP.Equal(dstIPParsed) {
			continue // 不是目标发来的包，忽略（同一 raw socket 可能收到其他无关 TCP 流量）
		}
		if n < 20 {
			continue
		}
		hdr := buf[:n]
		gotSrcPort := binary.BigEndian.Uint16(hdr[0:2])
		gotDstPort := binary.BigEndian.Uint16(hdr[2:4])
		gotAck := binary.BigEndian.Uint32(hdr[8:12])
		flags := hdr[13]
		if gotSrcPort != uint16(dstPort) || gotDstPort != srcPort || gotAck != seq+1 {
			continue // 不是这次探测对应的回包
		}

		latency := time.Since(start).Milliseconds()

		if flags&tcpFlagSYN != 0 && flags&tcpFlagACK != 0 {
			// 收到 SYN/ACK：主动发 RST 立即结束，不完成三次握手，不建立真正连接。
			rst := buildRSTSegment(srcIP, dstIPParsed, srcPort, uint16(dstPort), seq+1)
			_, _ = conn.WriteTo(rst, dstAddr)
			return latency, nil
		}
		if flags&tcpFlagRST != 0 {
			// 目标端口没监听或主动拒绝：也算收到了回应，只是端口不可达，仍然是一次有效的 RTT 测量。
			return latency, nil
		}
	}
}

// outboundIPv4 通过一次 UDP "连接"（不会真的发包，只是让内核选路）来确定
// 到达 dst 时会使用的本机出口 IPv4 地址，用于构造 TCP 伪首部算校验和。
func outboundIPv4(dst net.IP) (net.IP, error) {
	conn, err := net.Dial("udp4", net.JoinHostPort(dst.String(), "80"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.To4() == nil {
		return nil, fmt.Errorf("could not determine outbound IPv4 address")
	}
	return addr.IP.To4(), nil
}

func buildTCPHeader(srcPort, dstPort uint16, seq, ack uint32, flags byte, window uint16) []byte {
	const headerLen = 20
	hdr := make([]byte, headerLen)
	binary.BigEndian.PutUint16(hdr[0:2], srcPort)
	binary.BigEndian.PutUint16(hdr[2:4], dstPort)
	binary.BigEndian.PutUint32(hdr[4:8], seq)
	binary.BigEndian.PutUint32(hdr[8:12], ack)
	hdr[12] = byte((headerLen / 4) << 4) // data offset：5 个 32 位字，不带 TCP 选项
	hdr[13] = flags
	binary.BigEndian.PutUint16(hdr[14:16], window)
	// hdr[16:18] 校验和，调用方填充；hdr[18:20] 紧急指针恒为 0
	return hdr
}

// buildSYNSegment 构造一个 SYN 包，并在 TCP 头后面填充零字节把整个 TCP 段
// 撑到 totalSize（默认约 1400 字节），跟 mtr `--tcp -s 1400` 是同一种手法：
// 用大包去验证链路对大包的处理情况（分片/MTU黑洞/限速策略），
// 不是为了传输真实业务数据。
func buildSYNSegment(srcIP, dstIP net.IP, srcPort, dstPort uint16, seq uint32, totalSize int) []byte {
	hdr := buildTCPHeader(srcPort, dstPort, seq, 0, tcpFlagSYN, 65535)
	if totalSize < len(hdr) {
		totalSize = len(hdr)
	}
	segment := make([]byte, totalSize)
	copy(segment, hdr)
	setTCPChecksum(segment, srcIP, dstIP)
	return segment
}

func buildRSTSegment(srcIP, dstIP net.IP, srcPort, dstPort uint16, seq uint32) []byte {
	hdr := buildTCPHeader(srcPort, dstPort, seq, 0, tcpFlagRST, 0)
	setTCPChecksum(hdr, srcIP, dstIP)
	return hdr
}

// setTCPChecksum 按 RFC793 用 IPv4 伪首部计算 TCP 校验和，写回 segment[16:18]。
func setTCPChecksum(segment []byte, srcIP, dstIP net.IP) {
	binary.BigEndian.PutUint16(segment[16:18], 0)

	pseudo := make([]byte, 12+len(segment))
	copy(pseudo[0:4], srcIP.To4())
	copy(pseudo[4:8], dstIP.To4())
	pseudo[8] = 0
	pseudo[9] = 6 // TCP 协议号
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(segment)))
	copy(pseudo[12:], segment)

	binary.BigEndian.PutUint16(segment[16:18], internetChecksum(pseudo))
}

func internetChecksum(data []byte) uint16 {
	var sum uint32
	n := len(data)
	for i := 0; i+1 < n; i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if n%2 == 1 {
		sum += uint32(data[n-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
