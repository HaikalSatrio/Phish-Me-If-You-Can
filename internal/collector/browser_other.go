//go:build !windows

package collector

import "os/exec"

func configureBrowser(cmd *exec.Cmd) {}
