package util

import (
	"os"

	logging "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/go-logging"
)

func init() {
	SetupLogging()
}

var log = Logger("util")

var ansiGray = "\033[0;37m"
var ansiBlue = "\033[0;34m"

// LogFormats is a map of formats used for our logger, keyed by name.
var LogFormats = map[string]string{
	"nocolor": "%{time:2006-01-02 15:04:05.000000} %{level} %{module} %{shortfile}: %{message}",
	"color": ansiGray + "%{time:15:04:05.000} %{color}%{level:5.5s} " + ansiBlue +
		"%{module:10.10s}: %{color:reset}%{message} " + ansiGray + "%{shortfile}%{color:reset}",
}
var defaultLogFormat = "color"

// Logging environment variables
const (
	envLogging    = "IPFS_LOGGING"
	envLoggingFmt = "IPFS_LOGGING_FMT"
)

// loggers is the set of loggers in the system
var loggers = map[string]*logging.Logger{}

// SetupLogging will initialize the logger backend and set the flags.
func SetupLogging() {

	fmt := LogFormats[os.Getenv(envLoggingFmt)]
	if fmt == "" {
		fmt = LogFormats[defaultLogFormat]
	}

	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(backend)
	logging.SetFormatter(logging.MustStringFormatter(fmt))

	lvl := logging.ERROR

	if logenv := os.Getenv(envLogging); logenv != "" {
		var err error
		lvl, err = logging.LogLevel(logenv)
		if err != nil {
			log.Debugf("logging.LogLevel() Error: %q", err)
			lvl = logging.ERROR // reset to ERROR, could be undefined now(?)
		}
	}

	Debug = GetenvBool("IPFS_DEBUG")
	if Debug {
		lvl = logging.DEBUG
	}

	SetAllLoggers(lvl)

}

// SetDebugLogging calls SetAllLoggers with logging.DEBUG
func SetDebugLogging() {
	SetAllLoggers(logging.DEBUG)
}

// SetAllLoggers changes the logging.Level of all loggers to lvl
func SetAllLoggers(lvl logging.Level) {
	logging.SetLevel(lvl, "")
	for n, _ := range loggers {
		logging.SetLevel(lvl, n)
	}
}

// Logger retrieves a particular logger
func Logger(name string) *logging.Logger {
	log := logging.MustGetLogger(name)
	log.ExtraCalldepth = 1
	loggers[name] = log
	return log
}

// SetLogLevel changes the log level of a specific subsystem
// name=="*" changes all subsystems
func SetLogLevel(name, level string) error {
	lvl, err := logging.LogLevel(level)
	if err != nil {
		return err
	}

	// wildcard, change all
	if name == "*" {
		SetAllLoggers(lvl)
		return nil
	}

	// Check if we have a logger by that name
	// logging.SetLevel() can't tell us...
	_, ok := loggers[name]
	if !ok {
		return ErrNoSuchLogger
	}

	logging.SetLevel(lvl, name)

	return nil
}
