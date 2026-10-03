// Package keychain stores one password per server: macOS Keychain, Windows DPAPI, nothing elsewhere.
package keychain

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

// Set pipes hex to `security -i`, keeping the password out of argv; ponytail: Security.framework once Developer ID signed.
func Set(id, pw string) error {
	cmd := exec.Command("security", "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -a soteria -s soteria:%s -w hex:%x\n", id, pw))
	return cmd.Run()
}

func Get(id string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-a", "soteria", "-s", "soteria:"+id, "-w").Output()
	if err != nil {
		return "", err
	}
	s := strings.TrimSuffix(string(out), "\n")
	if h, ok := strings.CutPrefix(s, "hex:"); ok {
		b, err := hex.DecodeString(h)
		return string(b), err
	}
	return s, nil
}

func Delete(id string) {
	_ = exec.Command("security", "delete-generic-password", "-a", "soteria", "-s", "soteria:"+id).Run()
}
