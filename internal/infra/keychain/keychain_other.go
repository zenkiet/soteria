//go:build !darwin && !windows

package keychain

import "errors"

func Set(string, string) error {
	return errors.New("saving passwords is not supported on this platform")
}

func Get(string) (string, error) {
	return "", errors.New("saving passwords is not supported on this platform")
}
func Delete(string) {}
