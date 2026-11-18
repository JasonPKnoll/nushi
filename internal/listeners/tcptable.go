package listeners

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

// Windows' listening TCP sockets as iphlpapi's GetExtendedTcpTable tells
// them (TCP_TABLE_OWNER_PID_LISTENER), which listeners_windows.go asks for:
// a MIB_TCPTABLE_OWNER_PID for IPv4 and a MIB_TCP6TABLE_OWNER_PID for
// IPv6, each a count and that many rows. The parsers build everywhere so
// their tests run on Linux and the Mac too; Windows is little-endian on
// each of its CPUs, so the counts, states and pids are read that way.
const (
	// tcpRowLen is a MIB_TCPROW_OWNER_PID: state, local address and
	// port, remote address and port, owning pid; each a DWORD.
	tcpRowLen = 24
	// tcp6RowLen is a MIB_TCP6ROW_OWNER_PID: local address (16 bytes),
	// scope and port; remote address, scope and port; state, owning pid.
	tcp6RowLen = 56
	// mibTCPStateListen is MIB_TCP_STATE_LISTEN.
	mibTCPStateListen = 2
)

// ownedSocket is one listening TCP socket and the process that owns it,
// as a row of the table tells them.
type ownedSocket struct {
	Addr netip.AddrPort
	Pid  int
}

// errShortTable is a table whose count says more rows than it holds.
var errShortTable = errors.New("GetExtendedTcpTable: the table is shorter than its count")

// parseTCPTable reads a MIB_TCPTABLE_OWNER_PID (v6 false) or a
// MIB_TCP6TABLE_OWNER_PID (v6 true): each row's local address, its port
// (in network order, in the DWORD's first two bytes; the other two may
// hold anything) and its owning pid. A row not listening, or with no
// port, is skipped; a v4-mapped address comes back as IPv4.
func parseTCPTable(b []byte, v6 bool) ([]ownedSocket, error) {
	if len(b) < 4 {
		return nil, errShortTable
	}
	count := binary.LittleEndian.Uint32(b)
	rowLen := tcpRowLen
	if v6 {
		rowLen = tcp6RowLen
	}
	rows := b[4:]
	// Compared before it is an int, which on 32-bit Windows a count of
	// 2^31 or more would wrap below 0.
	if uint64(count) > uint64(len(rows)/rowLen) {
		return nil, errShortTable
	}
	n := int(count)
	out := make([]ownedSocket, 0, n)
	for i := range n {
		r := rows[i*rowLen : (i+1)*rowLen]
		var (
			a          netip.Addr
			port       uint16
			state, pid uint32
		)
		if v6 {
			a = netip.AddrFrom16([16]byte(r[0:16])).Unmap()
			port = binary.BigEndian.Uint16(r[20:22])
			state, pid = binary.LittleEndian.Uint32(r[48:52]), binary.LittleEndian.Uint32(r[52:56])
		} else {
			a = netip.AddrFrom4([4]byte(r[4:8]))
			port = binary.BigEndian.Uint16(r[8:10])
			state, pid = binary.LittleEndian.Uint32(r[0:4]), binary.LittleEndian.Uint32(r[20:24])
		}
		if state != mibTCPStateListen || port == 0 {
			continue
		}
		out = append(out, ownedSocket{Addr: netip.AddrPortFrom(a, port), Pid: int(pid)})
	}
	return out, nil
}

// tableListeners makes the Listeners a connection to localhost can reach
// of the two tables' sockets. Windows lists a dual-stack socket (an IPv6
// wildcard without IPV6_V6ONLY: Go's ":p", Node's default) in both, as
// 0.0.0.0:p and [::]:p, and says nothing else of its IPV6_V6ONLY; so a
// process's [::]:p beside its own 0.0.0.0:p is listed once, as a
// dual-stack [::]:p. Were the two a process's IPv4 and v6-only sockets
// (Windows lets the two families' wildcards bind the same port), that one
// Listener takes what they take: each family's connections, to that
// process.
func tableListeners(v4, v6 []ownedSocket) []Listener {
	type pidPort struct {
		pid  int
		port uint16
	}
	any4 := map[pidPort]int{} // how many of each process's 0.0.0.0:p
	for _, s := range v4 {
		if s.Addr.Addr().IsUnspecified() {
			any4[pidPort{s.Pid, s.Addr.Port()}]++
		}
	}
	dual := map[pidPort]int{} // the 0.0.0.0:p a dual-stack [::]:p stands for
	var six []Listener
	for _, s := range v6 {
		l := Listener{Pid: s.Pid, Addr: s.Addr}
		if k := (pidPort{s.Pid, s.Addr.Port()}); l.Addr.Addr().Is6() && l.Addr.Addr().IsUnspecified() && any4[k] > dual[k] {
			dual[k]++
			l.DualStack = true
		}
		if l.local() {
			six = append(six, l)
		}
	}
	var out []Listener
	for _, s := range v4 {
		if k := (pidPort{s.Pid, s.Addr.Port()}); s.Addr.Addr().IsUnspecified() && dual[k] > 0 {
			dual[k]--
			continue
		}
		if l := (Listener{Pid: s.Pid, Addr: s.Addr}); l.local() {
			out = append(out, l)
		}
	}
	return append(out, six...)
}
