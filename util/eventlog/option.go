package eventlog

import (
	"io"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/gopkg.in/natefinch/lumberjack.v2"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/maybebtc/logrus"
)

type Option func()

func Configure(options ...Option) {
	for _, f := range options {
		f()
	}
}

// LdJSONFormatter formats the event log as line-delimited JSON
var LdJSONFormatter = func() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

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

var LevelDebug = func() {
	logrus.SetLevel(logrus.DebugLevel)
}

var LevelError = func() {
	logrus.SetLevel(logrus.ErrorLevel)
}

var LevelInfo = func() {
	logrus.SetLevel(logrus.InfoLevel)
}
