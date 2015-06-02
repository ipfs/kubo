// +build linux darwin freebsd openbsd

package pb

import "syscall"

const sys_ioctl = syscall.SYS_IOCTL
