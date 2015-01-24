package component

import (
	"os"
	"path"

	config "github.com/jbenet/go-ipfs/repo/config"
	dir "github.com/jbenet/go-ipfs/thirdparty/dir"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

func InitEventlogComponent(repoPath string, conf *config.Config) error {
	if err := dir.Writable(path.Join(repoPath, "logs")); err != nil {
		return err
	}
	return nil
}

func EventlogComponentIsInitialized(path string) bool {
	return true
}

type EventlogComponent struct {
	path string
}

func (c *EventlogComponent) SetPath(path string) {
	c.path = path // FIXME necessary?
}

func (c *EventlogComponent) Close() error {
	// TODO It isn't part of the current contract, but callers may like for us
	// to disable logging once the component is closed.
	eventlog.Configure(eventlog.Output(os.Stderr))
	return nil
}

func (c *EventlogComponent) Open() error {
	// log.Debugf("writing eventlogs to ...", c.path)
	return configureEventLoggerAtRepoPath(c.path)
}

func configureEventLoggerAtRepoPath(repoPath string) error {
	eventlog.Configure(eventlog.LevelInfo)
	eventlog.Configure(eventlog.LdJSONFormatter)
	rotateConf := eventlog.LogRotatorConfig{
		Filename: path.Join(repoPath, "logs", "events.log"),
	}
	eventlog.Configure(eventlog.OutputRotatingLogFile(rotateConf))
	return nil
}
