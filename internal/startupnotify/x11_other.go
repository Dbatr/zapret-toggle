//go:build !linux || !cgo || (wayland && !x11)

package startupnotify

// Без X11 сообщение отправлять некому: Complete только чистит окружение.
func broadcast([]byte) error { return nil }
