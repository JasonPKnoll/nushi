package logs

import (
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

var (
	stdMu sync.Mutex
	// stdHandle is the copy of the log's handle that standard output and
	// error are now, closed once a newer file's copy takes its place.
	stdHandle windows.Handle
)

// toStd makes the process's standard output and error handles a copy of
// f's, which the runtime looks up for each write of a crash. Windows has
// no dup2: os.Stdout and os.Stderr keep the handles the process started
// with, so only the crash, and what asks for the standard handles, lands
// in the log.
func toStd(f *os.File) error {
	stdMu.Lock()
	defer stdMu.Unlock()
	self := windows.CurrentProcess()
	var h windows.Handle
	if err := windows.DuplicateHandle(self, windows.Handle(f.Fd()), self, &h, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return err
	}
	for _, std := range []uint32{windows.STD_OUTPUT_HANDLE, windows.STD_ERROR_HANDLE} {
		if err := windows.SetStdHandle(std, h); err != nil {
			_ = windows.CloseHandle(h) // the old copy is still the handle
			return err
		}
	}
	if stdHandle != 0 {
		_ = windows.CloseHandle(stdHandle) // the old file's copy, no longer standard
	}
	stdHandle = h
	return nil
}

// OpenAppend opens path for appending, made if missing, shared for
// delete as well as read and write: os.OpenFile's handle doesn't let
// another rename the file while it's open, and rotate renames the log it
// writes to (a relaunched daemon's stderr holds it too: hideConsole).
func OpenAppend(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	h, err := windows.CreateFile(p, windows.FILE_APPEND_DATA|windows.SYNCHRONIZE, share, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
