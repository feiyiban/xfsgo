package xfsvm

import (
	"bytes"
	"testing"
)

var (
	testRow = []byte{
		0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff,
		0x01,
	}
	testStringRows = []byte{
		0x68, 0x65, 0x6c, 0x6c,
		0x6f, 0x2c, 0x20, 0x77,
		0x6f, 0x72, 0x6c, 0x64,
		0x00, 0x00, 0x00, 0x00,
	}
)

func TestBuffer_ReadRow(t *testing.T) {
	buf := NewBuffer(testRow)
	r, err := buf.ReadRow()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("r=%x", r[:])
}

func TestBuffer_ReadString(t *testing.T) {
	want := []byte("hello, world")
	buf := NewBuffer(testStringRows)
	r, err := buf.ReadString(len(want))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want[:], r) {
		t.Fatalf("want=%x, got=%x", want, []byte(r))
	}
}
