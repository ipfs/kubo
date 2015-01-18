package repo

import (
	config "github.com/jbenet/go-ipfs/repo/config"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	util "github.com/jbenet/go-ipfs/util"
)

func ConfigureEventLogger(config config.Logs) error {
	if util.Debug {
		eventlog.Configure(eventlog.LevelDebug)
	} else {
		eventlog.Configure(eventlog.LevelInfo)
	}
	eventlog.Configure(eventlog.LdJSONFormatter)
	rotateConf := eventlog.LogRotatorConfig{
		Filename:   config.Filename,
		MaxSizeMB:  config.MaxSizeMB,
		MaxBackups: config.MaxBackups,
		MaxAgeDays: config.MaxAgeDays,
	}
	eventlog.Configure(eventlog.OutputRotatingLogFile(rotateConf))
	return nil
}
