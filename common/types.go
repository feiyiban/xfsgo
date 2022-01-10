// Copyright 2018 The xfsgo Authors
// This file is part of the xfsgo library.
//
// The xfsgo library is free software: you can redistribute it and/or modify
// it under the terms of the MIT Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The xfsgo library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// MIT Lesser General Public License for more details.
//
// You should have received a copy of the MIT Lesser General Public License
// along with the xfsgo library. If not, see <https://mit-license.org/>.

package common

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	AddrLen               = 25
	HashLen               = 32
	DefaultAddressVersion = 1
)

type (
	Hash    [HashLen]byte
	Address [AddrLen]byte
)

var (
	ZeroHash        = Bytes2Hash([]byte{})
	HashZ           = Hash{}
	AddrCheckSumLen = 4
	ZeroAddr        = Address{}
)

func Hex2bytes(s string) []byte {
	if len(s) > 1 {
		if s[0:2] == "0x" {
			s = s[2:]
		}
		if len(s)%2 == 1 {
			s = "0" + s
		}
		bs, err := hex.DecodeString(s)
		if err != nil {
			fmt.Printf("Hex2bytes:%v", err)
			return nil
		}
		return bs
	}
	return nil
}

func Bytes2Hash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

func Hex2Hash(s string) Hash {
	return Bytes2Hash(Hex2bytes(s))
}

func (h *Hash) SetBytes(other []byte) {
	for i, v := range other {
		h[i] = v
	}
}

func (h *Hash) Hex() string {
	return "0x" + hex.EncodeToString(h[:])
}
func (h *Hash) Bytes() []byte {
	return h[:]
}

func Bytes2Address(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}
func B58ToAddress(enc []byte) Address {
	return Bytes2Address(B58Decode(enc))
}
func StrB58ToAddress(enc string) Address {
	return B58ToAddress([]byte(enc))
}

func Hex2Address(s string) Address {
	return Bytes2Address(Hex2bytes(s))
}

func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddrLen:]
	}
	copy(a[AddrLen-len(b):], b)
}

func (a *Address) Hex() string {
	if bytes.Equal(a[:], ZeroAddr[:]) {
		return ""
	}
	return "0x" + hex.EncodeToString(a[:])
}

func (a *Address) Bytes() []byte {
	return a[:]
}

func (a *Address) String() string {
	if bytes.Equal(a[:], ZeroAddr[:]) {
		return ""
	}
	return a.B58String()
}

func (a *Address) B58() []byte {
	if bytes.Equal(a[:], ZeroAddr[:]) {
		return nil
	}
	return B58Encode(a.Bytes())
}

func (a *Address) Version() uint8 {
	return a[0]
}
func (a *Address) PubKeyHash() []byte {
	return a[1 : AddrLen-AddrCheckSumLen]
}
func (a *Address) Payload() []byte {
	return a[:AddrLen-AddrCheckSumLen]
}
func (a *Address) Checksum() []byte {
	return a[1+(AddrLen-AddrCheckSumLen)-1:]
}

func (a *Address) B58String() string {
	return string(a.B58())
}

func (a *Address) Hash() Hash {
	return Bytes2Hash(a[:])
}

func (a *Address) Equals(b Address) bool {
	return bytes.Compare(a.Bytes(), b.Bytes()) == Zero
}

func (h *Hash) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", h.Hex())), nil
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	if data == nil || len(data) < HashLen {
		h.SetBytes([]byte{0})
		return nil
	}
	hash := Hex2Hash(string(data[1 : len(data)-1]))
	h.SetBytes(hash.Bytes())
	return nil
}

func (a *Address) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", a.B58())), nil
}

func (a *Address) UnmarshalJSON(data []byte) error {
	if data == nil || len(data) < 28 {
		a.SetBytes([]byte{0})
		return nil
	}
	b58a := B58ToAddress(data[1 : len(data)-1])
	a.SetBytes(b58a.Bytes())
	return nil
}

func AddrCalibrator(val string) error {
	addr := B58Decode([]byte(val))

	if len(addr) != AddrLen {
		return errors.New("parameter byte length rule failed")
	}
	return nil
}

func HashCalibrator(val string) error {
	hash := Hex2bytes(val)
	if len(hash) != HashLen {
		return errors.New("parameter byte length rule failed")
	}
	return nil
}
