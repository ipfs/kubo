// Systemd readiness notification (sd_notify). Linux only.
//go:build linux

package kubo

import (
	daemon "github.com/coreos/go-systemd/v22/daemon"
)

func notifyReady() {
	_, _ = daemon.SdNotify(false, daemon.SdNotifyReady)
}

func notifyStopping() {
	_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)
}
