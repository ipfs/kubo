package utp

import (
	"log"
	"os"
	"strconv"
)

type logger struct {
	level int
}

var ulog *logger

func init() {
	logenv := os.Getenv("GO_UTP_LOGGING")

	var level int
	if len(logenv) > 0 {
		l, err := strconv.Atoi(logenv)
		if err != nil {
			log.Print("warning: GO_UTP_LOGGING must be numeric")
		} else {
			level = l
		}
	}

	ulog = &logger{level}
}

func (l *logger) Print(level int, v ...interface{}) {
	if l.level < level {
		return
	}
	log.Print(v...)
}

func (l *logger) Printf(level int, format string, v ...interface{}) {
	if l.level < level {
		return
	}
	log.Printf(format, v...)
}

func (l *logger) Println(level int, v ...interface{}) {
	if l.level < level {
		return
	}
	log.Println(v...)
}
