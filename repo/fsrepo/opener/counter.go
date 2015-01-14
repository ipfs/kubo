package fsrepo

import "path"

// TODO this could be made into something more generic.

type Counter struct {
	// repos maps repo paths to the number of openers holding an FSRepo handle
	// to it
	repos map[string]int
}

func NewCounter() *Counter {
	return &Counter{
		repos: make(map[string]int),
	}
}

// NumOpeners returns the number of FSRepos holding a handle to the repo at
// this path. This method is not thread-safe. The caller must have this object
// locked.
func (l *Counter) NumOpeners(repoPath string) int {
	return l.repos[key(repoPath)]
}

// AddOpener messages that an FSRepo holds a handle to the repo at this path.
// This method is not thread-safe. The caller must have this object locked.
func (l *Counter) AddOpener(repoPath string) error {
	l.repos[key(repoPath)]++
	return nil
}

// RemoveOpener messgaes that an FSRepo no longer holds a handle to the repo at
// this path. This method is not thread-safe. The caller must have this object
// locked.
func (l *Counter) RemoveOpener(repoPath string) error {
	l.repos[key(repoPath)]--
	return nil
}

func key(repoPath string) string {
	return path.Clean(repoPath)
}
