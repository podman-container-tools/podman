//go:build linux || freebsd

package main

import (
	"log/syslog"

	"github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

func syslogHook() {
	if !registry.PodmanConfig().Syslog {
		return
	}

	hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")
	if err != nil {
		logrus.Debug("Failed to initialize syslog hook: " + err.Error())
	} else {
		logrus.AddHook(hook)
	}
}
