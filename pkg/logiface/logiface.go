package logiface

type Logger interface {
	Errorf(format string, args ...any)
	Debugf(format string, args ...any)
}

var logger Logger

func SetLogger(l Logger) {
	logger = l
}

func Errorf(format string, args ...any) {
	if logger != nil {
		logger.Errorf(format, args...)
	}
}

func Debugf(format string, args ...any) {
	if logger != nil {
		logger.Debugf(format, args...)
	}
}
