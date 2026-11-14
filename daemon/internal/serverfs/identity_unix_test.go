//go:build !windows

package serverfs

import "os"

func currentTestIdentity() (int, int) {
	return os.Getuid(), os.Getgid()
}
