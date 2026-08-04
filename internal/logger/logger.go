package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/itportal/it-portal-agent/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(level string) (*zap.Logger, error) {
	var parsed zapcore.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	fileConfig := zap.NewProductionEncoderConfig()
	fileConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	file, err := os.OpenFile(filepath.Join(config.RootDir(), "logs", "agent.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(fileConfig), zapcore.AddSync(file), parsed)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
