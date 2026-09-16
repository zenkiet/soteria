package wails

import "strings"

// Deep links mirror SPA routes: soteria://files/<path> opens the app at /files/<path>.
// The payload stays percent-encoded end to end; the router decodes it.

func linkRoute(u string) string {
	route, ok := strings.CutPrefix(u, "soteria:/")
	if !ok || !strings.HasPrefix(route, "/files/") {
		return ""
	}
	return route
}

// openLink shows the window at the linked folder. Before sign-in the SPA would
// land on an error page, so the route is parked instead and the frontend picks
// it up with DeepLink once it has connected.
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
