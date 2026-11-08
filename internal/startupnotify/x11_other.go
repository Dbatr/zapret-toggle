//go:build !linux || !cgo

package startupnotify

func broadcast([]byte) error { return nil }
