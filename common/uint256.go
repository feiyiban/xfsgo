package common

import (
	"encoding/binary"

	"github.com/holiman/uint256"
)

// Bytes25 returns the value of z as a 25-byte big-endian array.
func UInt256_2Bytes25(z *uint256.Int) [25]byte {
	// The PutUint64()s are inlined and we get 4x (load, bswap, store) instructions.
	var b [25]byte
	binary.BigEndian.PutUint64(b[0:1], z[3])
	binary.BigEndian.PutUint64(b[1:9], z[2])
	binary.BigEndian.PutUint64(b[9:17], z[1])
	binary.BigEndian.PutUint64(b[17:25], z[0])
	return b
}
