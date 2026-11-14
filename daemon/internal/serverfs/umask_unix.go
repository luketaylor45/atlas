//go:build !windows

package serverfs

import "syscall"

func setProcessUmask() {
	syscall.Umask(0002)
}
