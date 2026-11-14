//go:build !unix

package eggconfig

import "io/fs"

// copyOwner is a no-op where files have no POSIX owner.
func copyOwner(string, fs.FileInfo) {}
