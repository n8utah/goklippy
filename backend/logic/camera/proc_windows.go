//go:build windows

package camera

import (
	"os/exec"
	"strconv"
	"syscall"
)

// hideWindow keeps ffmpeg from opening a console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// killTree stops the process and everything it started. An ffmpeg on PATH can be a
// launcher (a Python package's ffmpeg.exe) that runs the real ffmpeg as a child;
// Windows doesn't stop children with their parent.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	hideWindow(kill)
	if err := kill.Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
