// Package engine supervises a local inference engine process so a node
// is one command: the agent spawns the engine, restarts it if it dies,
// and tears it down on shutdown.
package engine

import (
	"context"
	"log"
	"os/exec"
	"time"
)

// Supervisor runs an engine command in a restart loop.
type Supervisor struct {
	NewCmd       func() *exec.Cmd // builds a fresh command per (re)start
	RestartDelay time.Duration
}

// Run keeps the engine alive until ctx is cancelled, then terminates the
// child and returns.
func (s *Supervisor) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		cmd := s.NewCmd()
		if err := cmd.Start(); err != nil {
			log.Printf("engine: start: %v", err)
		} else {
			log.Printf("engine: started pid %d", cmd.Process.Pid)
			exited := make(chan error, 1)
			go func() { exited <- cmd.Wait() }()

			select {
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				<-exited
				return
			case err := <-exited:
				log.Printf("engine: exited: %v — restarting in %s", err, s.RestartDelay)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(s.RestartDelay):
		}
	}
}
