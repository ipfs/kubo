package util

import (
	"os"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

func init() {
	SetupLogging()
}

var log = logrus.New()

// LogFormats is a map of formats used for our logger, keyed by name.
// TODO: write custom TextFormatter (don't print module=name explicitly) and
// fork logrus to add shortfile
var LogFormats = map[string]*logrus.TextFormatter{
	"nocolor": &logrus.TextFormatter{DisableColors: true, FullTimestamp: true, TimestampFormat: "2006-01-02 15:04:05.000000", DisableSorting: true},
	"color":   &logrus.TextFormatter{DisableColors: false, FullTimestamp: true, TimestampFormat: "15:04:05:000", DisableSorting: true},
}
var defaultLogFormat = "color"

// Logging environment variables
const (
	envLogging    = "IPFS_LOGGING"
	envLoggingFmt = "IPFS_LOGGING_FMT"
)

// loggers is the set of loggers in the system
var loggers = map[string]*logrus.Entry{}

// SetupLogging will initialize the logger backend and set the flags.
func SetupLogging() {

	format, ok := LogFormats[os.Getenv(envLoggingFmt)]
	if !ok {
		format = LogFormats[defaultLogFormat]
	}

	log.Out = os.Stderr
	log.Formatter = format

	lvl := logrus.ErrorLevel

	if logenv := os.Getenv(envLogging); logenv != "" {
		var err error
		lvl, err = logrus.ParseLevel(logenv)
		if err != nil {
			log.Debugf("logrus.ParseLevel() Error: %q", err)
			lvl = logrus.ErrorLevel // reset to ERROR, could be undefined now(?)
		}
	}

	if Debug := GetenvBool("IPFS_DEBUG"); Debug {
		lvl = logrus.DebugLevel
	}

	SetAllLoggers(lvl)
}

// SetDebugLogging calls SetAllLoggers with logrus.DebugLevel
func SetDebugLogging() {
	SetAllLoggers(logrus.DebugLevel)
}

// SetAllLoggers changes the logrus.Level of all loggers to lvl
func SetAllLoggers(lvl logrus.Level) {
	log.Level = lvl
	for _, logger := range loggers {
		logger.Level = lvl
	}
}

// Logger retrieves a particular logger
func Logger(name string) *logrus.Entry {
	if len(name) == 0 {
		log.Warnf("Missing name parameter")
		name = "undefined"
	}
	if _, ok := loggers[name]; !ok {
		loggers[name] = log.WithField("module", name)
	}
	return loggers[name]
}

// SetLogLevel changes the log level of a specific subsystem
// name=="*" changes all subsystems
func SetLogLevel(name, level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}

	// wildcard, change all
	if name == "*" {
		SetAllLoggers(lvl)
		return nil
	}

	// Check if we have a logger by that name
	if _, ok := loggers[name]; !ok {
		return ErrNoSuchLogger
	}

	loggers[name].Level = lvl

	return nil
}
