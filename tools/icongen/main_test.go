package main

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// The .ico's layout: a header, an entry for each image naming its size
// (256 written 0), its length and where it starts, then the images in order.
func TestIco(t *testing.T) {
	images := [][]byte{[]byte("sixteen"), []byte("two-five-six!")}
	b, err := ico([]int{16, 256}, images)
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if got := []uint16{le.Uint16(b), le.Uint16(b[2:]), le.Uint16(b[4:])}; got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("header %v, want [0 1 2]", got)
	}
	for i, side := range []byte{16, 0} {
		e := b[6+16*i:]
		if e[0] != side || e[1] != side || le.Uint16(e[4:]) != 1 || le.Uint16(e[6:]) != 32 {
			t.Errorf("entry %d: % x", i, e[:8])
		}
		size, at := le.Uint32(e[8:]), le.Uint32(e[12:])
		if got := b[at : at+size]; !bytes.Equal(got, images[i]) {
			t.Errorf("image %d is %q, want %q", i, got, images[i])
		}
	}
	if want := 6 + 2*16 + 7 + 13; len(b) != want {
		t.Errorf("%d bytes, want %d", len(b), want)
	}

	if _, err := ico([]int{512}, images[:1]); err == nil {
		t.Error("a 512 px icon was packed")
	}
	if _, err := ico([]int{16}, images); err == nil {
		t.Error("one size for two images was packed")
	}
}
