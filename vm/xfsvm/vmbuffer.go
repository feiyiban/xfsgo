package xfsvm

import (
	"errors"
	"io"
)

const smallBufferSize = 64

var ErrTooLarge = errors.New("bytes.Buffer: too large")

const maxInt = int(^uint(0) >> 1)

type Buffer interface {
	ReadUint8() (CTypeUint8, error)
	ReadUint16() (CTypeUint16, error)
	ReadUint32() (CTypeUint32, error)
	ReadString(size int) (CTypeString, error)
	ReadUint256() (CTypeUint256, error)
	Write(p []byte) (n int, err error)
	Bytes() []byte
}

type buffer struct {
	buf []byte
	off int
}
type row [8]byte

var rowlen = len(row{})

func (b *buffer) Bytes() []byte { return b.buf[b.off:] }
func (b *buffer) empty() bool   { return len(b.buf) <= b.off }
func (b *buffer) Len() int      { return len(b.buf) - b.off }
func (b *buffer) Reset() {
	b.buf = b.buf[:0]
	b.off = 0
}

func (b *buffer) tryGrowByReslice(n int) (int, bool) {
	if l := len(b.buf); n <= cap(b.buf)-l {
		b.buf = b.buf[:l+n]
		return l, true
	}
	return 0, false
}
func makeSlice(n int) []byte {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(ErrTooLarge)
		}
	}()
	return make([]byte, n)
}
func (b *buffer) grow(n int) int {
	m := b.Len()
	// If buffer is empty, reset to recover space.
	if m == 0 && b.off != 0 {
		b.Reset()
	}
	//// Try to grow by means of a reslice.
	if i, ok := b.tryGrowByReslice(n); ok {
		return i
	}
	if b.buf == nil && n <= smallBufferSize {
		b.buf = make([]byte, n, smallBufferSize)
		return 0
	}
	c := cap(b.buf)
	if n <= c/2-m {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= c to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		copy(b.buf, b.buf[b.off:])
	} else if c > maxInt-c-n {
		panic(ErrTooLarge)
	} else {
		// Not enough space anywhere, we need to allocate.
		buf := makeSlice(2*c + n)
		copy(buf, b.buf[b.off:])
		b.buf = buf
	}
	// Restore b.off and len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]
	return m
}
func (b *buffer) Write(p []byte) (n int, err error) {
	blocks := len(p) / rowlen
	mod := len(p) % rowlen
	if mod != 0 {
		blocks += 1
	}
	m, ok := b.tryGrowByReslice(blocks * rowlen)
	if !ok {
		m = b.grow(blocks * rowlen)
	}
	return copy(b.buf[m:], p), nil
}

func (b *buffer) ReadRow() (row, error) {
	if b.empty() {
		// Buffer is empty, reset to recover space.
		b.Reset()
		return row{}, io.EOF
	}
	var rowvar row
	n := copy(rowvar[:], b.buf[b.off:])
	if n < len(row{}) {
		return row{}, io.EOF
	}
	b.off += n
	return rowvar, nil
}
func (b *buffer) ReadRows(size int) ([]row, int, error) {
	blocks := size / rowlen
	mod := size % rowlen
	if mod != 0 {
		blocks += 1
	}
	var buf = make([]row, 0)
	for i := 0; i < blocks; i++ {
		in, err := b.ReadRow()
		if err != nil {
			return nil, 0, err
		}
		buf = append(buf, in)
	}
	return buf, mod, nil
}

func (b *buffer) ReadUint8() (n CTypeUint8, e error) {
	var r row
	r, e = b.ReadRow()
	if e != nil {
		return
	}
	return CTypeUint8{r[0]}, nil
}

func (b *buffer) ReadUint16() (n CTypeUint16, e error) {
	var r row
	r, e = b.ReadRow()
	if e != nil {
		return
	}
	copy(n[:], r[:2])
	return
}

func (b *buffer) ReadUint32() (n CTypeUint32, e error) {
	var r row
	r, e = b.ReadRow()
	if e != nil {
		return
	}
	copy(n[:], r[:4])
	return
}
func (b *buffer) ReadUint256() (n CTypeUint256, e error) {
	var r []row
	var m int
	r, m, e = b.ReadRows(len(n))
	if e != nil {
		return
	}
	buf := make([]byte, len(r)*rowlen)
	for i := 0; i < len(r); i++ {
		start := i * rowlen
		end := (i * rowlen) + rowlen
		copy(buf[start:end], r[i][:])
	}
	if len(buf) > rowlen {
		copy(n[:], buf[:len(buf)-m])
		return
	}
	copy(n[:], buf[:m])
	return
}
func (b *buffer) ReadString(size int) (n CTypeString, e error) {
	var r []row
	var m int
	r, m, e = b.ReadRows(size)
	if e != nil {
		return
	}
	buf := make([]byte, len(r)*rowlen)
	for i := 0; i < len(r); i++ {
		start := i * rowlen
		end := (i * rowlen) + rowlen
		copy(buf[start:end], r[i][:])
	}
	if len(buf) > rowlen {
		n = buf[:len(buf)-m]
		return
	}
	n = buf[:m]
	return
}

func NewBuffer(data []byte) *buffer {
	return &buffer{
		buf: data,
	}
}
