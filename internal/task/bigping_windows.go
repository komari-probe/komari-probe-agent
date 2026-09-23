//go:build windows

package task

import (
	"context"
	"errors"
	"time"
)

// BigPacketProbe 在 Windows 上不提供。Windows 自 XP SP2 起限制普通程序通过
// raw socket 发送自定义 TCP 包，要绕开就得引入 Npcap/WinPcap 之类的第三方
// 驱动，这会破坏探针"单文件、零外部依赖"的部署方式，所以这个手动大包
// 诊断功能只在 Linux/macOS 上提供，见 bigping_unix.go。
func BigPacketProbe(_ context.Context, _ string, _ time.Duration) (int64, error) {
	return -1, errors.New("big packet probe is not supported on windows")
}
