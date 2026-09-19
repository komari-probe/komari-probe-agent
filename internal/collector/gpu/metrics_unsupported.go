//go:build !linux && !windows

package gpu

import "errors"

// ModelNames 获取GPU型号信息 - 回退实现
func ModelNames() ([]string, error) {
	return nil, errors.New("detailed GPU monitoring not supported on this platform")
}

// Devices reports that detailed GPU metrics are unsupported on this platform.
func Devices() ([]Device, error) {
	return nil, errors.New("detailed GPU monitoring not supported on this platform")
}
