//go:build !windows

package acp

import (
	"os/exec"
	"testing"
)

func TestPrepareCommandStartsProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 1")
	prepareCommand(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("expected command to start a new process group: %#v", cmd.SysProcAttr)
	}
}
