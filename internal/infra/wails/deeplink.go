package wails

import "strings"

// soteria://files/<path> opens /files/<path>; the path stays percent-encoded and the router decodes it.
func linkRoute(u string) string {
	route, ok := strings.CutPrefix(u, "soteria:/")
	if !ok || !strings.HasPrefix(route, "/files/") {
		return ""
	}
	return route
}

// Before sign-in the route would land on an error page, so it is parked for DeepLink instead.
func (a *App) openLink(u string) {
	route := linkRoute(u)
	if route == "" {
		return
	}
	if a.S.Current() == nil {
		a.mu.Lock()
		a.link = route
		a.mu.Unlock()
		route = ""
	}
	a.show(route)
}

// DeepLink hands the parked route to the frontend once.
func (a *App) DeepLink() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.link
	a.link = ""
	return l
}
