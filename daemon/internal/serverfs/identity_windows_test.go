//go:build windows

package serverfs

func currentTestIdentity() (int, int) {
	return 1000, 1000
}
