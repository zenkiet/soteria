//go:build !windows

package wails

// Only Windows needs the workaround; macOS relaunches activate the running app themselves.
func notifyFirstInstance() {}
