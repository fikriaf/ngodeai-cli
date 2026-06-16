package logging

import (
	"fmt"
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger = slog.New(handler)
}

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }

func RecoverPanic(name string, cb func()) {
	if r := recover(); r != nil {
		logger.Error("Panic recovered", "name", name, "panic", fmt.Sprintf("%v", r))
		if cb != nil {
			cb()
		}
	}
}
