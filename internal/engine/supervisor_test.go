package engine

import (
	"context"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestRestartsCrashedEngine(t *testing.T) {
	var starts atomic.Int32
	s := &Supervisor{
		NewCmd: func() *exec.Cmd {
			starts.Add(1)
			return exec.Command("true") // exits immediately = crash
		},
		RestartDelay: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	s.Run(ctx)

	if n := starts.Load(); n < 3 {
		t.Errorf("expected repeated restarts of a crashing engine, got %d starts", n)
	}
}

func TestStopsChildOnCancel(t *testing.T) {
	s := &Supervisor{
		NewCmd: func() *exec.Cmd {
			return exec.Command("sleep", "30")
		},
		RestartDelay: 5 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // let the child start
	cancel()

	select {
	case <-done:
		// Run returned: the child was terminated, not waited for 30s
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop the child after context cancellation")
	}
}
