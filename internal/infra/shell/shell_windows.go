package shell

import (
	"os/exec"
	"syscall"
)

func Open(p string) error { return exec.Command("explorer.exe", p).Start() }

// Explorer parses its own command line: Go would quote the whole "/select,…" argument and Explorer ignores it.
func Reveal(p string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + p + `"`}
	return cmd.Start()
}
