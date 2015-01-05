package eventlog

import (
	"strings"

	"github.com/jbenet/go-ipfs/util"
)

// StandardLogger provides API compatibility with standard printf loggers
// eg. go-logging
type StandardLogger interface {
	Critical(args ...interface{})
	Criticalf(format string, args ...interface{})
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Notice(args ...interface{})
	Noticef(format string, args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
}

// StandardLogger provides API compatibility with standard printf loggers
// eg. go-logging
type PrefixLogger interface {
	StandardLogger

	Format() string
	Args() []interface{}

	Prefix(fmt string, args ...interface{}) PrefixLogger
}

// Logger retrieves an event logger by name
func Logger(system string) PrefixLogger {

	// TODO if we would like to adjust log levels at run-time. Store this event
	// logger in a map (just like the util.Logger impl)

	logger := util.Logger(system)
	return Prefix(logger, "")
}

func Prefix(l StandardLogger, format string, args ...interface{}) PrefixLogger {
	return &prefixLogger{logger: l, format: format, args: args}
}

type prefixLogger struct {
	logger StandardLogger
	format string
	args   []interface{}
}

func (pl *prefixLogger) Format() string {
	return pl.format
}

func (pl *prefixLogger) Args() []interface{} {
	return pl.args
}

func (pl *prefixLogger) Prefix(fmt string, args ...interface{}) PrefixLogger {
	return Prefix(pl, fmt, args...)
}

func (pl *prefixLogger) prepend(fmt string, args []interface{}) (string, []interface{}) {
	together := make([]interface{}, 0, len(pl.args)+len(args))
	together = append(together, pl.args...)
	together = append(together, args...)
	if len(pl.format) > 0 {
		fmt = pl.format + " " + fmt
	}
	return fmt, together
}

func valfmtn(count int) string {
	s := strings.Repeat("%v ", count)
	s = s[:len(s)-1] // remove last space
	return s
}

type logFunc func(args ...interface{})
type logFuncf func(fmt string, args ...interface{})

func (pl *prefixLogger) logFunc(f logFuncf, args ...interface{}) {
	// need to actually use the format version, with extra fmt strings appended
	fmt := valfmtn(len(args))
	pl.logFuncf(f, fmt, args...)
}

func (pl *prefixLogger) logFuncf(f logFuncf, format string, args ...interface{}) {
	format, args = pl.prepend(format, args)
	f(format, args...)
}

func (pl *prefixLogger) Critical(args ...interface{}) {
	pl.logFunc(pl.logger.Criticalf, args...)
}
func (pl *prefixLogger) Debug(args ...interface{}) {
	pl.logFunc(pl.logger.Debugf, args...)
}
func (pl *prefixLogger) Error(args ...interface{}) {
	pl.logFunc(pl.logger.Errorf, args...)
}
func (pl *prefixLogger) Fatal(args ...interface{}) {
	pl.logFunc(pl.logger.Fatalf, args...)
}
func (pl *prefixLogger) Info(args ...interface{}) {
	pl.logFunc(pl.logger.Infof, args...)
}
func (pl *prefixLogger) Notice(args ...interface{}) {
	pl.logFunc(pl.logger.Noticef, args...)
}
func (pl *prefixLogger) Panic(args ...interface{}) {
	pl.logFunc(pl.logger.Panicf, args...)
}
func (pl *prefixLogger) Warning(args ...interface{}) {
	pl.logFunc(pl.logger.Warningf, args...)
}

func (pl *prefixLogger) Criticalf(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Criticalf, format, args...)
}
func (pl *prefixLogger) Debugf(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Debugf, format, args...)
}
func (pl *prefixLogger) Errorf(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Errorf, format, args...)
}
func (pl *prefixLogger) Fatalf(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Fatalf, format, args...)
}
func (pl *prefixLogger) Infof(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Infof, format, args...)
}
func (pl *prefixLogger) Noticef(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Noticef, format, args...)
}
func (pl *prefixLogger) Panicf(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Panicf, format, args...)
}
func (pl *prefixLogger) Warningf(format string, args ...interface{}) {
	pl.logFuncf(pl.logger.Warningf, format, args...)
}
