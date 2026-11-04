//go:build unix

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcAttrs puts the agent in its own process group so cancel kills the
// whole tree (agents spawn shells, test runners, etc).
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
}
