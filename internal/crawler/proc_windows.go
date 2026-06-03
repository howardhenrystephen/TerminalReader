//go:build windows

package crawler

import "os/exec"

func setProcessGroup(cmd *exec.Cmd) {
	// Windows 不支持 Setpgid，留空
}
