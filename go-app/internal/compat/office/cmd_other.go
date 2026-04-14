//go:build !windows

package office

import "os/exec"

func applyNoWindow(_ *exec.Cmd) {}
