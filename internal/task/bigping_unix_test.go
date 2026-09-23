//go:build !windows

package task

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestBuildSYNSegmentSizeAndFlags(t *testing.T) {
	srcIP := net.ParseIP("10.0.0.1").To4()
	dstIP := net.ParseIP("10.0.0.2").To4()
	seg := buildSYNSegment(srcIP, dstIP, 12345, 80, 0xdeadbeef, bigPacketTotalSize)

	if len(seg) != bigPacketTotalSize {
		t.Fatalf("expected segment length %d, got %d", bigPacketTotalSize, len(seg))
	}
	if got := binary.BigEndian.Uint16(seg[0:2]); got != 12345 {
		t.Fatalf("src port = %d, want 12345", got)
	}
	if got := binary.BigEndian.Uint16(seg[2:4]); got != 80 {
		t.Fatalf("dst port = %d, want 80", got)
	}
	if got := binary.BigEndian.Uint32(seg[4:8]); got != 0xdeadbeef {
		t.Fatalf("seq = %x, want deadbeef", got)
	}
	if seg[13] != tcpFlagSYN {
		t.Fatalf("flags = %x, want SYN only (%x)", seg[13], tcpFlagSYN)
	}
}

func TestBuildSYNSegmentMinimumSize(t *testing.T) {
	srcIP := net.ParseIP("10.0.0.1").To4()
	dstIP := net.ParseIP("10.0.0.2").To4()
	// 要求的 totalSize 小于 TCP 头长度时，应至少保留完整的 20 字节头部。
	seg := buildSYNSegment(srcIP, dstIP, 1, 1, 1, 4)
	if len(seg) != 20 {
		t.Fatalf("expected segment to be clamped to 20 bytes, got %d", len(seg))
	}
}

func TestTCPChecksumVerifiesToZero(t *testing.T) {
	srcIP := net.ParseIP("192.168.1.10").To4()
	dstIP := net.ParseIP("192.168.1.20").To4()
	seg := buildSYNSegment(srcIP, dstIP, 55555, 443, 42, bigPacketTotalSize)

	// Internet 校验和的性质（RFC 1071 §4.1）：把已经算好校验和的报文（连同
	// 伪首部）再整体求一次校验和，S + ^S 在补码运算下恒等于 0xffff，取反后
	// 结果应该是全 0，不是全 1——这是接收端验证校验和的标准做法。
	pseudo := make([]byte, 12+len(seg))
	copy(pseudo[0:4], srcIP)
	copy(pseudo[4:8], dstIP)
	pseudo[8] = 0
	pseudo[9] = 6
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(seg)))
	copy(pseudo[12:], seg)

	if got := internetChecksum(pseudo); got != 0 {
		t.Fatalf("checksum self-verification failed: got %#x, want 0", got)
	}
}

func TestBuildRSTSegment(t *testing.T) {
	srcIP := net.ParseIP("10.0.0.1").To4()
	dstIP := net.ParseIP("10.0.0.2").To4()
	seg := buildRSTSegment(srcIP, dstIP, 100, 200, 999)
	if len(seg) != 20 {
		t.Fatalf("RST segment should be exactly 20 bytes (no payload), got %d", len(seg))
	}
	if seg[13] != tcpFlagRST {
		t.Fatalf("flags = %x, want RST only (%x)", seg[13], tcpFlagRST)
	}
}

func TestInternetChecksumOddLength(t *testing.T) {
	// 奇数长度的数据，最后一个字节要按高位对齐参与求和，这里只验证不 panic
	// 且结果是稳定的（同样输入两次算出同样结果）。
	data := []byte{0x01, 0x02, 0x03}
	a := internetChecksum(data)
	b := internetChecksum(data)
	if a != b {
		t.Fatalf("checksum not deterministic: %x vs %x", a, b)
	}
}
