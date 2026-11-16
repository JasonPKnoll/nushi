//go:build unix

package openflag

import "syscall"

const (
	// NoFollow opens a link as a link: the open fails with ELOOP.
	NoFollow = syscall.O_NOFOLLOW
	// NonBlock keeps an open of a fifo from waiting for its writer.
	NonBlock = syscall.O_NONBLOCK
	// Directory fails the open of anything but a folder.
	Directory = syscall.O_DIRECTORY
)
