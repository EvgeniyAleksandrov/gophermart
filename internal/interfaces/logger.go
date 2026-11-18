package interfaces

import "go.uber.org/zap"

// Logger provides structured logging without giving components control of process exit.
// *zap.Logger implements Logger directly; its owner is responsible for Sync.
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
}
