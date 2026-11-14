//go:build unix

package eggconfig

import (
	"io/fs"
	"os"
	"syscall"
)

// copyOwner gives path the owner of like. It is best effort: an unprivileged
// daemon cannot change ownership and does not need to.
func copyOwner(path string, like fs.FileInfo) {
	stat, ok := like.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	_ = os.Lchown(path, int(stat.Uid), int(stat.Gid))
}
