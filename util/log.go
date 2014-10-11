package util

import (
	"fmt"
	"os"

	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"
)

func init() {
	SetupLogging()
}

// LogFormat is the format used for our logger.
var LogFormat = "%{color}%{time:2006-01-02 15:04:05.999999} %{shortfile} %{level}: %{color:reset}%{message}"

// loggers is the set of loggers in the system
var loggers = map[string]*logging.Logger{}

// PErr is a shorthand printing function to output to Stderr.
func PErr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

// POut is a shorthand printing function to output to Stdout.
func POut(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, format, a...)
}

// SetupLogging will initialize the logger backend and set the flags.
func SetupLogging() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(backend)
	logging.SetFormatter(logging.MustStringFormatter(LogFormat))

	lvl := logging.ERROR

	var err error
	if logenv := os.Getenv("IPFS_LOGGING"); logenv != "" {
		lvl, err = logging.LogLevel(logenv)
		if err != nil {
			PErr("invalid logging level: %s\n", logenv)
			PErr("using logging.DEBUG\n")
			lvl = logging.DEBUG
		}
	}

	SetAllLoggers(lvl)
}

func SetAllLoggers(lvl logging.Level) {
	logging.SetLevel(lvl, "")
	for n, log := range loggers {
		logging.SetLevel(lvl, n)
		log.Error("setting logger: %s to %v", n, lvl)
	}
}

// Logger retrieves a particular logger + initializes it at a particular level
func Logger(name string) *logging.Logger {
	log := logging.MustGetLogger(name)
	// logging.SetLevel(lvl, name) // can't set level here.
	loggers[name] = log
	return log
}
