// Package shell opens paths in the OS file manager.
package shell

import "os/exec"

func Open(p string) error   { return exec.Command("open", p).Run() }
func Reveal(p string) error { return exec.Command("open", "-R", p).Run() }
