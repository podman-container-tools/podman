package rekor

import (
	"fmt"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/sirupsen/logrus"
)

// leveledLogger adapts our use of logrus to the expected go-retryablehttp.LeveledLogger interface.
type leveledLogger struct {
	logger *logrus.Logger
}

func leveledLoggerForLogrus(logger *logrus.Logger) retryablehttp.LeveledLogger {
	return &leveledLogger{logger: logger}
}

// log is the actual conversion implementation
func (l *leveledLogger) log(level logrus.Level, msg string, keysAndValues []any) {
	fields := logrus.Fields{}
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		key := keysAndValues[i]
		keyString, isString := key.(string)
		if !isString {
			// It seems attractive to panic() here, but we might already be in a failure state, so let’s not make it worse
			keyString = fmt.Sprintf("[Invalid LeveledLogger key %#v]", key)
		}
		fields[keyString] = keysAndValues[i+1]
	}
	l.logger.WithFields(fields).Log(level, msg)
}

// Debug implements retryablehttp.LeveledLogger
func (l *leveledLogger) Debug(msg string, keysAndValues ...any) {
	l.log(logrus.DebugLevel, msg, keysAndValues)
}

// Error implements retryablehttp.LeveledLogger
func (l *leveledLogger) Error(msg string, keysAndValues ...any) {
	l.log(logrus.ErrorLevel, msg, keysAndValues)
}

// Info implements retryablehttp.LeveledLogger
func (l *leveledLogger) Info(msg string, keysAndValues ...any) {
	l.log(logrus.InfoLevel, msg, keysAndValues)
}

// Warn implements retryablehttp.LeveledLogger
func (l *leveledLogger) Warn(msg string, keysAndValues ...any) {
	l.log(logrus.WarnLevel, msg, keysAndValues)
}
