//go:build windows

package main

import (
	"bytes"
	"fmt"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/svc/eventlog"
)

// Logrus hook that delegates to windows event log
type EventLogHook struct {
	events *eventlog.Log
}

type LogFormat struct {
	name string
}

func (f *LogFormat) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	fmt.Fprintf(b, "[%-5s] %s: %s", entry.Level.String(), f.name, entry.Message)

	for key, value := range entry.Data {
		fmt.Fprintf(b, " {%s = %s}", key, value)
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func NewEventHook(events *eventlog.Log, name string) *EventLogHook {
	logrus.SetFormatter(&LogFormat{name})
	return &EventLogHook{events}
}

func (hook *EventLogHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}

	switch entry.Level {
	case logrus.PanicLevel:
		return hook.events.Error(1002, line)
	case logrus.FatalLevel:
		return hook.events.Error(1001, line)
	case logrus.ErrorLevel:
		return hook.events.Error(1000, line)
	case logrus.WarnLevel:
		return hook.events.Warning(1000, line)
	case logrus.InfoLevel:
		return hook.events.Info(1000, line)
	case logrus.DebugLevel, logrus.TraceLevel:
		return hook.events.Info(1001, line)
	default:
		return nil
	}
}

func (hook *EventLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
