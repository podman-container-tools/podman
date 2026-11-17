# 🌈 slogcolor

[![Go Reference](https://pkg.go.dev/badge/github.com/SladkyCitron/slogcolor.svg)](https://pkg.go.dev/github.com/SladkyCitron/slogcolor) [![Go Report Card](https://goreportcard.com/badge/github.com/SladkyCitron/slogcolor)](https://goreportcard.com/report/github.com/SladkyCitron/slogcolor) [![Go](https://github.com/SladkyCitron/slogcolor/actions/workflows/go.yml/badge.svg)](https://github.com/SladkyCitron/slogcolor/actions/workflows/go.yml) [![GitHub license](https://img.shields.io/github/license/SladkyCitron/slogcolor)](https://github.com/SladkyCitron/slogcolor/blob/main/LICENSE) [![Made in Slovakia](https://raw.githubusercontent.com/pedromxavier/flag-badges/refs/heads/main/badges/SK.svg)](https://www.youtube.com/watch?v=UqXJ0ktrmh0)

![screenshot](https://github.com/SladkyCitron/slogcolor/blob/main/screenshot.png)

**slogcolor** is a little, customizable color handler for `log/slog`. It enhances log readability by color-coding log levels and supports flexible formatting options.
Its output is inspired by XMRig and zerolog.

## Installation

Run:

```sh
go get -u github.com/SladkyCitron/slogcolor
```

## Basic Usage

```go
package main

import (
    "os"
    "time"
    "errors"
    "log/slog"

    "github.com/SladkyCitron/slogcolor"
)

func main() {
    // Configure slog to use slogcolor by default for colored output
    slog.SetDefault(slog.New(slogcolor.NewHandler(os.Stderr, slogcolor.DefaultOptions)))

    slog.Info("Initializing")
    slog.Debug("Init done", "duration", 500*time.Millisecond)
    slog.Warn("Slow request!", "method", "GET", "path", "/api/users", "duration", 750*time.Millisecond)
    slog.Error("DB connection lost!", "err", errors.New("connection reset"), "db", "horalky")
}
```

### Default options

slogcolor provides a set of predefined options to simplify configuration. You can use these default options via [`DefaultOptions`](https://pkg.go.dev/github.com/SladkyCitron/slogcolor#DefaultOptions), or simply pass `nil` for the same effect.

```go
slog.SetDefault(slog.New(slogcolor.NewHandler(os.Stderr, slogcolor.DefaultOptions)))

// or

slog.SetDefault(slog.New(slogcolor.NewHandler(os.Stderr, nil)))
```

The behavior is identical in both cases, so you can choose based on your coding style or preference.

### Customized output format

The output format can also be customized using the [`Options`](https://pkg.go.dev/github.com/SladkyCitron/slogcolor#Options) like this:

```go
opts := &slogcolor.Options{
    Level:         slog.LevelDebug,
    TimeFormat:    time.RFC3339,
    SrcFileMode:   slog.Nop,
}
slog.SetDefault(slog.New(slogcolor.NewHandler(os.Stderr, opts)))
```

or like this:

```go
opts := slogcolor.DefaultOptions
opts.Level = slog.LevelDebug
opts.TimeFormat = time.RFC3339
opts.SrcFileMode = slog.Nop

slog.SetDefault(slog.New(slogcolor.NewHandler(os.Stderr, opts)))
```

### Prefixes

Prefixes can be useful for adding context to log messages, such as identifying different subsystems or components (e.g., `DB`, `SceneController`, `Network`) that generated the log.

Messages can be prefixed with [`Prefix`](https://pkg.go.dev/github.com/SladkyCitron/slogcolor#Prefix) like this:

```go
slog.Info(slogcolor.Prefix("MyPrefix", "kajšmentke"))
slog.Info(slogcolor.Prefix("SceneController", "switching scene"), "scene", "MainMenuScene")

// or

slog.Info(slogcolor.Prefix("MyPrefix")+"kajšmentke")
slog.Info(slogcolor.Prefix("SceneController")+"switching scene", "scene", "MainMenuScene")
```

It can also be used as an alias for ease of use, especially when you frequently use prefixes, like this:

```go
var P = slogcolor.Prefix

slog.Info(P("MyPrefix", "kajšmentke"))

// or

slog.Info(P("MyPrefix")+"kajšmentke")
```

### Disable colors

Colors are enabled by default but can be disabled using `Options.NoColor`. Particularly useful for automatically enabling colors based on the terminal capabilities using e.g. the [go-isatty](https://github.com/mattn/go-isatty) package.

```go
w := os.Stderr

opts := slogcolor.DefaultOptions
opts.NoColor = !isatty.IsTerminal(w.Fd())

slog.SetDefault(slog.New(slogcolor.NewHandler(w, opts)))
```

## License

Licensed under the **MIT License** (see [LICENSE](https://github.com/SladkyCitron/slogcolor/blob/main/LICENSE))
