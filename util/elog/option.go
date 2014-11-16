package elog

import (
	"io"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/gopkg.in/natefinch/lumberjack.v2"
)

type Option func()

func SetOption(o Option) {
	o()
}

var JSONFormatter = func() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

var TextFormatter = func() {
	logrus.SetFormatter(&logrus.TextFormatter{})
}

type LogRotatorConfig struct {
	File       string
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
				Filename:   config.File,
				MaxSize:    int(config.MaxSizeMB),
				MaxBackups: int(config.MaxBackups),
				MaxAge:     int(config.MaxAgeDays),
			})
	}
}

// TODO log levels?  logrus.SetLevel(logrus.DebugLevel)
