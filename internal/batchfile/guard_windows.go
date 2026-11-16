package batchfile

import (
	"os"
	"os/exec"
)

// Guard readies cmd to start on Windows: its environment gets
// NoCurrentDir, and a batch file it runs becomes Command's (node and its
// script, for an npm shim), or, when Command refuses, a cmd whose Start
// fails with Command's reason. A batch file kept as it is runs by its
// full path, so cmd.exe doesn't look its name up again.
func Guard(cmd *exec.Cmd) {
	if cmd.Env != nil {
		cmd.Env = Environ(cmd.Env)
	}
	if cmd.Err != nil || !Is(cmd.Path) {
		return
	}
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	path, args, err := Command(cmd.Path, cmd.Args[1:], env)
	if err != nil {
		cmd.Err = err
		return
	}
	cmd.Path, cmd.Args = path, append([]string{path}, args...)
}
