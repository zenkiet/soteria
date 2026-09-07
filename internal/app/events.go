// Package app holds the use cases. It talks to infra directly and reports to the UI only through Events.
package app

// Events is the one way out of this layer; the Wails adapter implements it, tests record it.
type Events interface {
	Emit(name string, data any)
}
