// Package logging provides the Agent's process-wide, level-aware logger.
package logging

import (
	"fmt"
	stdlog "log"
	"strings"
	"sync/atomic"
)

type level int32

const (
	debugLevel level = iota
	infoLevel
	warnLevel
	errorLevel
)

var currentLevel atomic.Int32

func init() { currentLevel.Store(int32(infoLevel)) }

// SetLevel configures the minimum emitted level.
func SetLevel(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		currentLevel.Store(int32(debugLevel))
	case "info", "":
		currentLevel.Store(int32(infoLevel))
	case "warn", "warning":
		currentLevel.Store(int32(warnLevel))
	case "error":
		currentLevel.Store(int32(errorLevel))
	default:
		return fmt.Errorf("unsupported log level %q (use debug, info, warn, or error)", value)
	}
	return nil
}

func Print(v ...any)                 { emit(infoLevel, fmt.Sprint(v...)) }
func Println(v ...any)               { emit(infoLevel, fmt.Sprintln(v...)) }
func Printf(format string, v ...any) { emit(infoLevel, fmt.Sprintf(format, v...)) }
func Debugln(v ...any)               { emit(debugLevel, fmt.Sprintln(v...)) }
func Debugf(format string, v ...any) { emit(debugLevel, fmt.Sprintf(format, v...)) }
func Warnln(v ...any)                { emit(warnLevel, fmt.Sprintln(v...)) }
func Warnf(format string, v ...any)  { emit(warnLevel, fmt.Sprintf(format, v...)) }
func Errorln(v ...any)               { emit(errorLevel, fmt.Sprintln(v...)) }
func Errorf(format string, v ...any) { emit(errorLevel, fmt.Sprintf(format, v...)) }

func emit(messageLevel level, message string) {
	if messageLevel >= level(currentLevel.Load()) {
		stdlog.Print(strings.TrimSuffix(message, "\n"))
	}
}
