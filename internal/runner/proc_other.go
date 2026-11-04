//go:build !unix

package runner

import (
	"os/exec"
	"time"
)

func setProcAttrs(cmd *exec.Cmd) { cmd.WaitDelay = 5 * time.Second }
