package harness

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"time"
)

// processTracker keeps track of all daemon processes started during tests
type processTracker struct {
	mu        sync.Mutex
	processes map[int]*os.Process
}

// globalProcessTracker is a package-level tracker for all spawned daemons
var globalProcessTracker = &processTracker{
	processes: make(map[int]*os.Process),
}

// registerProcess adds a process to the tracker
func (pt *processTracker) registerProcess(proc *os.Process) {
	if proc == nil {
		return
	}
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.processes[proc.Pid] = proc
	log.Debugf("registered daemon process PID %d", proc.Pid)
}

// unregisterProcess removes a process from the tracker
func (pt *processTracker) unregisterProcess(pid int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	delete(pt.processes, pid)
	log.Debugf("unregistered daemon process PID %d", pid)
}

// killAll forcefully terminates all tracked processes
func (pt *processTracker) killAll() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	count := len(pt.processes)
	if count == 0 {
		return
	}

	log.Debugf("cleaning up %d daemon processes", count)

	for pid, proc := range pt.processes {
		log.Debugf("force killing daemon process PID %d", pid)

		// Try SIGTERM first
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			if !isProcessDone(err) {
				log.Debugf("error sending SIGTERM to PID %d: %v", pid, err)
			}
		}

		// Give it a moment to terminate
		time.Sleep(100 * time.Millisecond)

		// Force kill if still running
		if err := proc.Kill(); err != nil {
			if !isProcessDone(err) {
				log.Debugf("error killing PID %d: %v", pid, err)
			}
		}

		// Clean up entry
		delete(pt.processes, pid)
	}
}

// isProcessDone checks if an error indicates the process has already exited
func isProcessDone(err error) bool {
	return errors.Is(err, os.ErrProcessDone)
}

// CleanupDaemonProcesses kills all tracked daemon processes
// This should be called in test cleanup or panic recovery
func CleanupDaemonProcesses() {
	globalProcessTracker.killAll()
}

// RegisterBackgroundProcess registers an external process for cleanup tracking
// This is useful for tests that start processes outside of the harness Runner
func RegisterBackgroundProcess(proc *os.Process) {
	if proc != nil {
		globalProcessTracker.registerProcess(proc)
	}
}
