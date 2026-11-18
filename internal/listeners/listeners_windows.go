package listeners

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// getExtendedTCPTable is iphlpapi's GetExtendedTcpTable, which x/sys
// doesn't wrap; the system's own DLL, by its full path (no cgo).
var getExtendedTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

// tcpTableOwnerPidListener is TCP_TABLE_OWNER_PID_LISTENER: the listening
// sockets alone, each with the pid that owns it.
const tcpTableOwnerPidListener = 3

// List reads the TCP listeners that a connection to localhost can reach
// (on a loopback address or a wildcard), from GetExtendedTcpTable's IPv4
// and IPv6 tables (tableListeners). Each socket is listed once, with the
// process that owns it (the one that opened it), not each one holding it:
// Windows doesn't tie a socket to the processes it was handed to. Every
// user's listeners are listed, as the table holds them all: one of a
// service's, a connection to localhost may reach too.
func List() ([]Listener, error) {
	var socks [2][]ownedSocket
	for i, af := range []uint32{windows.AF_INET, windows.AF_INET6} {
		b, err := tcpTable(af)
		if errors.Is(err, windows.ERROR_NOT_SUPPORTED) && af == windows.AF_INET6 {
			break // no IPv6 on this system
		}
		if err == nil {
			socks[i], err = parseTCPTable(b, af == windows.AF_INET6)
		}
		if err != nil {
			return nil, fmt.Errorf("listing listening sockets: %w", err)
		}
	}
	return tableListeners(socks[0], socks[1]), nil
}

// tcpTable reads family af's table of listening sockets and their pids,
// growing the buffer while the table outgrows it (a few times at most: it
// can grow between the calls).
func tcpTable(af uint32) ([]byte, error) {
	if err := getExtendedTCPTable.Find(); err != nil { // Call would panic
		return nil, fmt.Errorf("GetExtendedTcpTable: %w", err)
	}
	size := uint32(4096)
	for range 4 {
		b := make([]byte, size)
		r, _, _ := getExtendedTCPTable.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(af), tcpTableOwnerPidListener, 0) //nolint:gosec // the call's own buffer and its size
		switch err := windows.Errno(r); {
		case err == 0:
			return b, nil
		case err != windows.ERROR_INSUFFICIENT_BUFFER:
			return nil, fmt.Errorf("GetExtendedTcpTable: %w", err)
		}
		size += 1024 // room for a few more sockets that open meanwhile
	}
	return nil, errors.New("GetExtendedTcpTable: the table kept growing")
}
