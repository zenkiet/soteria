// Package keychain stores one password per server: macOS Keychain, Windows DPAPI, nothing elsewhere.
package keychain

import (
	"os/exec"
	"strings"
)

func Set(id, pw string) error {
	return exec.Command("security", "add-generic-password", "-U", "-a", "soteria", "-s", "soteria:"+id, "-w", pw).Run()
}

func Get(id string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-a", "soteria", "-s", "soteria:"+id, "-w").Output()
	return strings.TrimSpace(string(out)), err
}

func Delete(id string) {
	_ = exec.Command("security", "delete-generic-password", "-a", "soteria", "-s", "soteria:"+id).Run()
}
