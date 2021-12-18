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

package assert

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/hex"
	"math/big"
	"reflect"
	"testing"
	"xfsgo/common"
	"xfsgo/crypto"
)

func IsEqual(val1, val2 interface{}) bool {
	v1 := reflect.ValueOf(val1)
	v2 := reflect.ValueOf(val2)

	if v1.Kind() == reflect.Ptr {
		v1 = v1.Elem()
	}

	if v2.Kind() == reflect.Ptr {
		v2 = v2.Elem()
	}

	if !v1.IsValid() && !v2.IsValid() {
		return true
	}

	switch v1.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if v1.IsNil() {
			v1 = reflect.ValueOf(nil)
		}
	}

	switch v2.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if v2.IsNil() {
			v2 = reflect.ValueOf(nil)
		}
	}

	v1Underlying := reflect.Zero(reflect.TypeOf(v1)).Interface()
	v2Underlying := reflect.Zero(reflect.TypeOf(v2)).Interface()

	if v1 == v1Underlying {
		if v2 == v2Underlying {
			goto CASE4
		} else {
			goto CASE3
		}
	} else {
		if v2 == v2Underlying {
			goto CASE2
		} else {
			goto CASE1
		}
	}

CASE1:
	return reflect.DeepEqual(v1.Interface(), v2.Interface())
CASE2:
	return reflect.DeepEqual(v1.Interface(), v2)
CASE3:
	return reflect.DeepEqual(v1, v2.Interface())
CASE4:
	return reflect.DeepEqual(v1, v2)
}
func Error(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func Equal(t *testing.T, got, want interface{}) {
	if !IsEqual(got, want) {
		t.Fatalf("got: %v want: %v\n", got, want)
	}
}

func True(t *testing.T, value bool, msgAndArgs ...interface{}) bool {
	if !value {
		// if h, ok := t.(tHelper); ok {
		// 	h.Helper()
		// }
		t.Fatalf("Should be true:%v\n", msgAndArgs...)
		return false
	}

	return true

}
func VerifyAddress(t *testing.T, addr common.Address) {
	if !crypto.VerifyAddress(addr) {
		t.Fatalf("got checksum: %v not verify\n", addr.Checksum())
	}
}
func AddressEq(t *testing.T, got common.Address, want common.Address) {
	if !got.Equals(want) {
		t.Fatalf("got: %v want: %v\n", got, want)
	}
}

func Nil(t *testing.T, err error) {
	t.Fatalf(err.Error())
}

// func NotNil
func HashEqual(t *testing.T, got, want common.Hash) {
	if bytes.Compare(got.Bytes(), want.Bytes()) != common.Zero {
		t.Fatalf("got: %x want: %x\n", got, want)
	}
}

func BytesEqual(t *testing.T, got, want []byte) {
	if got == nil || want == nil {
		t.Fatal("nil err")
	}
	if bytes.Compare(got, want) != common.Zero {
		t.Fatalf("got: %x want: %x\n", got, want)
	}
}

func BigIntEqual(t *testing.T, got, want *big.Int) {
	if got == nil || want == nil {
		t.Fatal("nil err")
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("got: %s want: %x\n", got.Bytes(), want.Bytes())
	}
}

func PrivateKeyEqual(t *testing.T, got *ecdsa.PrivateKey, want *ecdsa.PrivateKey) {
	if got == nil || want == nil {
		t.Fatal("nil err")
	}

	gotDer, err := x509.MarshalECPrivateKey(got)
	if err != nil {
		t.Fatal(err)
	}
	wantDer, err := x509.MarshalECPrivateKey(want)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(gotDer, wantDer) != common.Zero {
		gotHex := hex.EncodeToString(gotDer)
		wantHex := hex.EncodeToString(wantDer)
		t.Fatalf("got hex: %s, want hex: %s", gotHex, wantHex)
	}
}
