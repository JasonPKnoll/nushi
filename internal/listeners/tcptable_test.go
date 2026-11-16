package listeners

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/afrase/nushi/internal/testcheck"
)

// tcpRow is a MIB_TCPROW_OWNER_PID; junk goes in the local port's DWORD's
// upper two bytes, which Windows doesn't promise to clear.
func tcpRow(state uint32, addr string, port, junk uint16, pid uint32) []byte {
	r := make([]byte, tcpRowLen)
	binary.LittleEndian.PutUint32(r[0:], state)
	a := netip.MustParseAddr(addr).As4()
	copy(r[4:8], a[:])
	binary.BigEndian.PutUint16(r[8:], port)
	binary.LittleEndian.PutUint16(r[10:], junk)
	copy(r[12:16], []byte{10, 0, 0, 9}) // the remote address, all 0 for a listener; not read
	binary.LittleEndian.PutUint32(r[20:], pid)
	return r
}

// tcp6Row is a MIB_TCP6ROW_OWNER_PID.
func tcp6Row(state uint32, addr string, port uint16, pid uint32) []byte {
	r := make([]byte, tcp6RowLen)
	a := netip.MustParseAddr(addr).As16()
	copy(r[0:16], a[:])
	binary.LittleEndian.PutUint32(r[16:], 3) // a scope, not read
	binary.BigEndian.PutUint16(r[20:], port)
	binary.LittleEndian.PutUint32(r[48:], state)
	binary.LittleEndian.PutUint32(r[52:], pid)
	return r
}

// table is a count and its rows.
func table(rows ...[]byte) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(rows))) //nolint:gosec // a test's few rows
	return slices.Concat(append([][]byte{b}, rows...)...)
}

func TestParseTCPTable(t *testing.T) {
	v4, err := parseTCPTable(table(
		tcpRow(mibTCPStateListen, "127.0.0.1", 7990, 0xbeef, 4120),
		tcpRow(mibTCPStateListen, "0.0.0.0", 445, 0, 4),
		tcpRow(5, "127.0.0.1", 7991, 0, 4120), // ESTABLISHED: not a listener
		tcpRow(mibTCPStateListen, "127.0.0.1", 0, 0, 4120),
	), false)
	testcheck.NoErr(t, err)
	want := []ownedSocket{{netip.MustParseAddrPort("127.0.0.1:7990"), 4120}, {netip.MustParseAddrPort("0.0.0.0:445"), 4}}
	if !slices.Equal(v4, want) {
		t.Errorf("IPv4: %v, want %v", v4, want)
	}
	v6, err := parseTCPTable(table(
		tcp6Row(mibTCPStateListen, "::1", 7991, 4120),
		tcp6Row(mibTCPStateListen, "::", 7996, 4120),
		tcp6Row(mibTCPStateListen, "::ffff:127.0.0.1", 8080, 77),
	), true)
	testcheck.NoErr(t, err)
	want = []ownedSocket{{netip.MustParseAddrPort("[::1]:7991"), 4120}, {netip.MustParseAddrPort("[::]:7996"), 4120}, {netip.MustParseAddrPort("127.0.0.1:8080"), 77}}
	if !slices.Equal(v6, want) {
		t.Errorf("IPv6: %v, want %v", v6, want)
	}
}

func TestParseTCPTableShort(t *testing.T) {
	for name, b := range map[string][]byte{
		"empty":          nil,
		"a row short":    table(tcpRow(mibTCPStateListen, "127.0.0.1", 1, 0, 1))[:4+tcpRowLen-1],
		"count too high": append(binary.LittleEndian.AppendUint32(nil, 2), tcpRow(mibTCPStateListen, "127.0.0.1", 1, 0, 1)...),
	} {
		if _, err := parseTCPTable(b, false); !errors.Is(err, errShortTable) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if socks, err := parseTCPTable(table(), true); err != nil || len(socks) != 0 {
		t.Errorf("no rows: %v, %v", socks, err)
	}
}

// TestTableListeners takes the rows Windows 11 listed on 2026-11-17 for
// Go's five kinds of listener in one process (a dual-stack ":p" is in both
// tables), and made-up ones for the rest.
func TestTableListeners(t *testing.T) {
	own := func(addr string, pid int) ownedSocket { return ownedSocket{netip.MustParseAddrPort(addr), pid} }
	at := func(pid int, addr string, dual bool) Listener {
		return Listener{Pid: pid, Addr: netip.MustParseAddrPort(addr), DualStack: dual}
	}
	v4 := []ownedSocket{
		own("127.0.0.1:7990", 9), own("0.0.0.0:7992", 9), own("0.0.0.0:7996", 9), // Go's tcp4 127.0.0.1, tcp4 0.0.0.0, tcp ":0"
		own("0.0.0.0:3000", 20),     // another process's, beside 21's v6-only [::]:3000
		own("192.168.7.20:8080", 9), // another interface
		own("127.0.0.2:8081", 9),    // another loopback address
		own("0.0.0.0:5000", 30),     // a process's IPv4 wildcard and IPv6-only one: read as one dual-stack
		own("0.0.0.0:5001", 31),     // two IPv4 wildcards of one process (SO_REUSEADDR), one IPv6
		own("0.0.0.0:5001", 31),
	}
	v6 := []ownedSocket{
		own("[::1]:7991", 9), own("[::]:7993", 9), own("[::]:7996", 9), // tcp6 ::1, tcp6 [::], tcp ":0"
		own("[::]:3000", 21),
		own("[fe80::9]:8080", 9),
		own("[::]:5000", 30),
		own("[::]:5001", 31),
	}
	want := []Listener{
		at(9, "127.0.0.1:7990", false), at(9, "0.0.0.0:7992", false), at(20, "0.0.0.0:3000", false), at(31, "0.0.0.0:5001", false),
		at(9, "[::1]:7991", false), at(9, "[::]:7993", false), at(9, "[::]:7996", true), at(21, "[::]:3000", false),
		at(30, "[::]:5000", true), at(31, "[::]:5001", true),
	}
	if got := tableListeners(v4, v6); !slices.Equal(got, want) {
		t.Errorf("listed\n%v\nwant\n%v", got, want)
	}
}
