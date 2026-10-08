# Syslog Hooks for Logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/>

## Usage

```go
package main

import (
	"log/syslog"

	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func main() {
	log := logrus.New()
	hook, err := lsyslog.NewSyslogHook("udp", "localhost:514", syslog.LOG_INFO, "")

	if err == nil {
		log.Hooks.Add(hook)
	}
}
```

If you want to connect to local syslog (Ex. "/dev/log" or "/var/run/syslog" or "/var/run/log"). Just assign empty string to the first two parameters of `NewSyslogHook`. It should look like the following.

```go
package main

import (
	"log/syslog"

	"github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

func main() {
	log := logrus.New()
	hook, err := lsyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")

	if err == nil {
		log.Hooks.Add(hook)
	}
}
```

### Different log levels for local and remote logging

By default `NewSyslogHook()` sends logs through the hook for all log levels. If you want to have
different log levels between local logging and syslog logging (i.e., respect the `priority` argument
passed to `NewSyslogHook()`), you need to define a custom hook type that satisfies the `logrus.Hook`
interface by embedding `*lsyslog.SyslogHook` and overriding `Levels()` to return only the log levels
you're interested in.

The following example shows how to log at **DEBUG** level for local logging and **WARN** level for
syslog logging:

```go
package main

import (
	"log/syslog"

	log "github.com/sirupsen/logrus"
	lsyslog "github.com/sirupsen/logrus/hooks/syslog"
)

type customHook struct {
	*lsyslog.SyslogHook
}

func (h *customHook) Levels() []log.Level {
	return []log.Level{log.WarnLevel}
}

func main() {
	log.SetLevel(log.DebugLevel)

	hook, err := lsyslog.NewSyslogHook("tcp", "localhost:5140", syslog.LOG_WARNING, "myTag")
	if err != nil {
		panic(err)
	}

	log.AddHook(&customHook{hook})

	// ...
}
```
