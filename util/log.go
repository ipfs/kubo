package util

import (
	"fmt"
	"os"

	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"
)

func init() {
	SetupLogging()
}

var log = Logger("util")

// LogFormat is the format used for our logger.
var LogFormat = "%{color}%{time:2006-01-02 15:04:05.999999} %{shortfile} %{level}: %{color:reset}%{message}"

// loggers is the set of loggers in the system
var loggers = map[string]*logging.Logger{}

// POut is a shorthand printing function to output to Stdout.
func POut(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, format, a...)
}

// SetupLogging will initialize the logger backend and set the flags.
func SetupLogging() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(backend)
	logging.SetFormatter(logging.MustStringFormatter(LogFormat))

	// always prnt critical and error?
	SetAllLoggers(logging.CRITICAL)
	SetAllLoggers(logging.ERROR)

	if logenv := os.Getenv("IPFS_LOGGING"); logenv != "" {
		lvl, err := logging.LogLevel(logenv)
		if err != nil {
			log.Error("invalid logging level: %s\n", logenv)
		} else {
			SetAllLoggers(lvl)
		}
	}

	if GetenvBool("IPFS_DEBUG") {
		SetAllLoggers(logging.DEBUG)
	}

}

// SetAllLoggers changes the logging.Level of all loggers to lvl
func SetAllLoggers(lvl logging.Level) {
	logging.SetLevel(lvl, "")
	for n, log := range loggers {
		logging.SetLevel(lvl, n)
		log.Debug("setting logger: %s to %v", n, lvl)
	}
}

// Logger retrieves a particular logger + initializes it at a particular level
func Logger(name string) *logging.Logger {
	log := logging.MustGetLogger(name)
	loggers[name] = log
	return log
}
