// Package store is everything Soteria keeps on the local disk: the config folder, saved servers, window geometry, log, thumbnail cache.
package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"soteria/internal/domain"
)

// Dir is <UserConfigDir>/Soteria.
func Dir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "Soteria")
}

func ThumbDir() string { return filepath.Join(Dir(), "thumbs") }

func serversFile() string { return filepath.Join(Dir(), "servers.json") }

func LoadServers() ([]domain.Server, error) {
	list := []domain.Server{}
	b, err := os.ReadFile(serversFile())
	if errors.Is(err, fs.ErrNotExist) {
		return list, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func writeServers(list []domain.Server) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	return os.WriteFile(serversFile(), b, 0o600)
}

func SaveServer(s domain.Server) error {
	list, err := LoadServers()
	if err != nil {
		return err
	}
	list = slices.DeleteFunc(list, func(x domain.Server) bool { return x.ID == s.ID })
	return writeServers(append([]domain.Server{s}, list...))
}

func RemoveServer(id string) error {
	list, err := LoadServers()
	if err != nil {
		return err
	}
	return writeServers(slices.DeleteFunc(list, func(x domain.Server) bool { return x.ID == id }))
}

func FindServer(id string) (domain.Server, bool) {
	list, _ := LoadServers()
	i := slices.IndexFunc(list, func(x domain.Server) bool { return x.ID == id })
	if i < 0 {
		return domain.Server{}, false
	}
	return list[i], true
}

// WindowState survives relaunches in <config>/window.json.
type WindowState struct {
	X, Y, W, H int
	Hinted     bool // "still running" notification already shown once
}

func windowFile() string { return filepath.Join(Dir(), "window.json") }

func LoadWindow() (s WindowState) {
	if b, err := os.ReadFile(windowFile()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func SaveWindow(s WindowState) {
	b, _ := json.Marshal(s)
	_ = os.MkdirAll(Dir(), 0o700)
	_ = os.WriteFile(windowFile(), b, 0o600)
}
