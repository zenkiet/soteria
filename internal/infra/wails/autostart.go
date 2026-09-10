package wails

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var errDevBuild = errors.New("launch at login needs an installed build")

func (a *App) LaunchAtLogin() bool {
	if a.Version == "" {
		return false
	}
	on, _ := application.Get().Autostart.IsEnabled()
	return on
}

func (a *App) SetLaunchAtLogin(on bool) error {
	if a.Version == "" {
		return errDevBuild
	}
	if !on {
		return application.Get().Autostart.Disable()
	}
	return application.Get().Autostart.Enable()
}
