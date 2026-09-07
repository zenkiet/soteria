package keychain

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"soteria/internal/infra/store"
)

// Passwords are wrapped with DPAPI (readable only by this Windows account) and kept next to servers.json.
func secretFile(id string) string { return filepath.Join(store.Dir(), "secret-"+id) }

func dpapi(in []byte, protect bool) ([]byte, error) {
	if len(in) == 0 {
		return nil, errors.New("empty secret")
	}
	blob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&blob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&blob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func Set(id, pw string) error {
	b, err := dpapi([]byte(pw), true)
	if err != nil {
		return err
	}
	return os.WriteFile(secretFile(id), []byte(base64.StdEncoding.EncodeToString(b)), 0o600)
}

func Get(id string) (string, error) {
	enc, err := os.ReadFile(secretFile(id))
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(string(enc))
	if err != nil {
		return "", err
	}
	b, err := dpapi(raw, false)
	return string(b), err
}

func Delete(id string) { _ = os.Remove(secretFile(id)) }
