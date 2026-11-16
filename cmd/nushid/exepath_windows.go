package main

import (
	"os"

	"github.com/afrase/nushi/internal/batchfile"
)

// noCurrentDirPrograms sets batchfile.NoCurrentDir in this process's
// environment, first thing: neither the daemon nor a subcommand, nor what
// either starts (each inherits it), runs a program from its current folder
// by a bare name. A chat's folder is a project's, and its node.exe or
// git.bat isn't the user's.
func noCurrentDirPrograms() { _ = os.Setenv(batchfile.NoCurrentDir, "1") }
