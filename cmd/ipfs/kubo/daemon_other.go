// No-op readiness notification on non-Linux platforms.
//go:build !linux

package kubo

func notifyReady() {}

func notifyStopping() {}
