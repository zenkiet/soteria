package shell

import "os/exec"

func Open(p string) error   { return exec.Command("explorer.exe", p).Start() }
func Reveal(p string) error { return exec.Command("explorer.exe", "/select,"+p).Start() }
