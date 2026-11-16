package claudehistory

import (
	"bytes"
	"io"
)

// lines reads a growing file's lines as they come, from where the last read
// stopped: off is how far it read, carry the start of a line not yet ended.
// A line longer than max is passed over, and counted.
type lines struct {
	off      int64
	carry    []byte
	skipping bool // within a line past max, or one begun before off
	skipped  int  // lines passed over
}

// readChunk is how much of a file one ReadAt takes.
const readChunk = 64 << 10

// read reads r from off up to end, giving each line it ends to line.
func (l *lines) read(r io.ReaderAt, end int64, maxLine int, line func([]byte)) error {
	buf := make([]byte, readChunk)
	for l.off < end {
		n, err := r.ReadAt(buf[:min(int64(len(buf)), end-l.off)], l.off)
		if n == 0 && err != nil {
			return err
		}
		l.off += int64(n)
		l.take(buf[:n], maxLine, line)
	}
	return nil
}

// take takes a piece of the file: each line it ends, and what follows its
// last newline as carry.
func (l *lines) take(b []byte, maxLine int, line func([]byte)) {
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			if !l.skipping && len(l.carry)+len(b) <= maxLine {
				l.carry = append(l.carry, b...)
			} else {
				l.carry, l.skipping = nil, true
			}
			return
		}
		ln := b[:i]
		b = b[i+1:]
		if l.skipping || len(l.carry)+len(ln) > maxLine {
			l.carry, l.skipping = nil, false
			l.skipped++
			continue
		}
		if len(l.carry) > 0 {
			ln = append(l.carry, ln...)
			l.carry = nil
		}
		line(ln)
	}
}
