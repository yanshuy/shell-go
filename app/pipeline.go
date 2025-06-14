package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
)

type ProcessGroup struct {
	pgid int
	pids []int
	mu   sync.Mutex
}

func NewProcessGroup() *ProcessGroup {
	return &ProcessGroup{
		pgid: -1,
		pids: make([]int, 0),
	}
}

func (pg *ProcessGroup) AddProcess(proc *os.Process) error {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	if pg.pgid == -1 {
		pg.pgid = proc.Pid
		if err := syscall.Setpgid(proc.Pid, proc.Pid); err != nil {
			return fmt.Errorf("failed to set process group: %w", err)
		}
	} else {
		if err := syscall.Setpgid(proc.Pid, pg.pgid); err != nil {
			return fmt.Errorf("failed to add to process group: %w", err)
		}
	}

	pg.processes = append(pg.processes, proc)
	return nil
}

func (pg *ProcessGroup) Signal(sig os.Signal) error {
	pg.mu.RLock()
	defer pg.mu.RUnlock()

	if pg.pgid == -1 {
		return errors.New("no process group set")
	}

	return syscall.Kill(-pg.pgid, sig.(syscall.Signal))
}

type Job struct {
	ID           int
	Pipeline     *Pipeline
	ProcessGroup *ProcessGroup
	Background   bool
	Status       JobStatus
	StartTime    time.Time
	completed    chan struct{}
}

type JobStatus int

const (
	JobRunning JobStatus = iota
	JobStopped
	JobCompleted
	JobTerminated
)

type Pipeline struct {
	Commands  []*Command
	exitCode  int
	completed chan struct{}
	err       error
}