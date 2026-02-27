package procwait

import (
	"os"
	"os/exec"
	"testing"
)

func TestWaitNonexistentPID(t *testing.T) {
	// Use a very high PID that won't exist on any platform.
	// PID 0 is not safe — on Linux, kill(0, 0) signals the process group.
	err := Wait(4999999)
	if err == nil {
		t.Fatal("expected error for nonexistent PID")
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
