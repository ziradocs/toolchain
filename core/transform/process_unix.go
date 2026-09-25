//go:build !windows

package transform

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Keep the filter and children in one process group so a timeout or output
// limit can close every inherited stdout/stderr pipe before Wait returns.
func configureBoundedFilterProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if err == syscall.ESRCH {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
}

// WaitDelay can return after the parent exits while a child still owns an
// inherited pipe. Kill the process group on every exit path, including a
// successful parent exit, before returning to the caller.
func cleanupFilterDescendants(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
