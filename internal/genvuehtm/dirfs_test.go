package genvuehtm

import (
	"io/fs"
	"os"
)

// dirFS is a folder as a file system, for the test's fixtures.
func dirFS(dir string) fs.FS { return os.DirFS(dir) }
