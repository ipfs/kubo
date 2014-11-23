package eventlog

import (
	"io"
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/maybebtc/logrus"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/gopkg.in/natefinch/lumberjack.v2"
)

// init sets up sane defaults
func init() {
	Configure(TextFormatter)
	Configure(Output(os.Stderr))
	// has the effect of disabling logging since we log event entries at Info
	// level by convention
	Configure(LevelError)
}

type Option func()

// Configure applies the provided options sequentially from left to right
func Configure(options ...Option) {
	for _, f := range options {
		f()
	}
}

// LdJSONFormatter Option formats the event log as line-delimited JSON
var LdJSONFormatter = func() {
	logrus.SetFormatter(&PoliteJSONFormatter{})
}

// TextFormatter Option formats the event log as human-readable plain-text
var TextFormatter = func() {
	logrus.SetFormatter(&logrus.TextFormatter{})
}

type LogRotatorConfig struct {
	Filename   string
	MaxSizeMB  uint64
	MaxBackups uint64
	MaxAgeDays uint64
}

func Output(w io.Writer) Option {
	return func() {
		logrus.SetOutput(w)
		// TODO return previous Output option
	}
}

func OutputRotatingLogFile(config LogRotatorConfig) Option {
	return func() {
		logrus.SetOutput(
			&lumberjack.Logger{
				Filename:   config.Filename,
				MaxSize:    int(config.MaxSizeMB),
				MaxBackups: int(config.MaxBackups),
				MaxAge:     int(config.MaxAgeDays),
			})
	}
}

// LevelDebug Option sets the log level to debug
var LevelDebug = func() {
	logrus.SetLevel(logrus.DebugLevel)
}

// LevelDebug Option sets the log level to error
var LevelError = func() {
	logrus.SetLevel(logrus.ErrorLevel)
}

// LevelDebug Option sets the log level to info
var LevelInfo = func() {
	logrus.SetLevel(logrus.InfoLevel)
}
