package openflag

import "syscall"

const (
	// NoFollow opens a link or junction as itself, not what it points to
	// (CreateFile's FILE_FLAG_OPEN_REPARSE_POINT, one of the high bits
	// os.OpenFile passes on): the open succeeds, and its stat says it's a
	// link, so a caller that keeps only a regular file or a folder refuses
	// it after the open rather than at it.
	NoFollow = syscall.FILE_FLAG_OPEN_REPARSE_POINT
	// NonBlock is nothing: Windows has no fifo in the file system to wait
	// on.
	NonBlock = 0
	// Directory is nothing: os.OpenFile opens a folder for reading as it
	// does a file, so the caller's read of its names is what fails on
	// anything else.
	Directory = 0
)
