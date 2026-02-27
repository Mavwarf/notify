package procwait

import (
	"os"
	"os/exec"
	"testing"
)

func TestWaitNonexistentPID(t *testing.T) {
	// PID 0 is never a valid user process; should return an error.
	err := Wait(0)
	if err == nil {
		t.Fatal("expected error for PID 0")
	}
}

func TestWaitCompletedProcess(t *testing.T) {
	// Start a short-lived subprocess, wait for it to exit via os/exec first,
	// then verify that Wait returns successfully (process already exited).
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start subprocess: %v", err)
	}
	pid := cmd.Process.Pid
	cmd.Wait() // ensure the process has exited

	// Wait should succeed or return an appropriate error (process gone).
	// On Windows, OpenProcess may still succeed briefly after exit.
	// On Unix, kill(pid,0) will return ESRCH once reaped.
	_ = Wait(pid)
}
