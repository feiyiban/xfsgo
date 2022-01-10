package xfsvm

import (
	"encoding/binary"
	"testing"
)

func makeOpNumUint32(op byte, u uint32) []byte {
	var num OpNum
	binary.LittleEndian.PutUint32(num[:], u)
	return append([]byte{op}, num[:]...)
}
func makeOpNumUint64(op byte, u uint64) []byte {
	var num OpNum
	binary.LittleEndian.PutUint64(num[:], u)
	return append([]byte{op}, num[:]...)
}
func makeOpNumString(op byte, u string) []byte {
	bs := []byte(u)
	max := len(OpNum{})
	n := len(bs) / max
	mod := len(bs) % max
	if mod != 0 {
		n += 1
	}
	out := make([]byte, 0)
	push32 := func(row []byte) {
		var num OpNum
		copy(num[:], row[:])
		out = append(out, append([]byte{op}, num[:]...)...)
	}
	for i := 0; i < n; i++ {
		if mod != 0 && i == n-1 {
			row := bs[i*max : (i*max)+mod]
			push32(row)
			break
		} else if mod == 0 {
			row := bs[i*max:]
			push32(row)
			break
		}
		row := bs[i*max : (i*max)+max]
		push32(row)
	}
	return out
}
func Test_makeOpNumString(t *testing.T) {
	d := makeOpNumString(OpPush, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	t.Logf("data: %v", d)
	//makeOpNumString(OpPush, "aaaaaaaa")
	//makeOpNumString(OpPush, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

}
func TestVmc_Exec(t *testing.T) {
	c := NewVMC()

	//data := NewBuffer(nil)
	//n, err := data.Write([]byte("hello"))
	//if err != nil {
	//	return
	//}
	_ = c

}
