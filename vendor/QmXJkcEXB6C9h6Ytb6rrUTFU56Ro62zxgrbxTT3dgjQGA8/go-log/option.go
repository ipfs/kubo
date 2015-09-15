package log

import (
	"io"
	"os"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

// init sets up sane defaults
func init() {
	Configure(TextFormatter)
	Configure(Output(os.Stderr))
	// has the effect of disabling logging since we log event entries at Info
	// level by convention
	Configure(LevelError)
}

// Global writer group for logs to output to
var WriterGroup = new(MirrorWriter)

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

func Output(w io.Writer) Option {
	return func() {
		logrus.SetOutput(w)
		// TODO return previous Output option
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
