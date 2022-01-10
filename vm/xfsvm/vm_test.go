package xfsvm

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"
	"xfsgo/assert"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/crypto"
)

type testStateTree struct {
	data  map[[32]byte][]byte
	codes map[[32]byte][]byte
	nonce map[[32]byte]uint64
}

func (t *testStateTree) GetNonce(addr common.Address) uint64 {
	if nonce, ok := t.nonce[ahash.SHA256Array(addr[:])]; ok {
		return nonce
	}
	return 0
}
func (t *testStateTree) GetBalance(common.Address) *big.Int {
	return nil
}

func (t *testStateTree) GetCode(addr common.Address) []byte {
	if code, ok := t.codes[ahash.SHA256Array(addr[:])]; ok {
		return code
	}
	return nil
}
func (t *testStateTree) SetState(addr common.Address, key [32]byte, v []byte) {
	k := ahash.SHA256Array(append(addr[:], key[:]...))
	t.data[k] = v
}
func (t *testStateTree) GetStateValue(addr common.Address, key [32]byte) []byte {
	k := ahash.SHA256Array(append(addr[:], key[:]...))
	if data, ok := t.data[k]; ok {
		return data
	}
	return nil
}

func (t *testStateTree) SetCode(addr common.Address, code []byte) {
	t.codes[ahash.SHA256Array(addr[:])] = code
}
func (t *testStateTree) AddNonce(addr common.Address, val uint64) {
	oldnonce, _ := t.nonce[ahash.SHA256Array(addr[:])]
	t.nonce[ahash.SHA256Array(addr[:])] = oldnonce + val
}
func newTestStateTree() *testStateTree {
	return &testStateTree{
		data:  make(map[[32]byte][]byte),
		codes: make(map[[32]byte][]byte),
		nonce: make(map[[32]byte]uint64),
	}
}

func readBytes4hex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
func mustReadBytes4hex(s string) []byte {
	bs, err := readBytes4hex(s)
	if err != nil {
		panic(err)
	}
	return bs
}
func writeStringParams(w Buffer, s CTypeString) {
	slen := len(s)
	var slenbuf [8]byte
	binary.LittleEndian.PutUint64(slenbuf[:], uint64(slen))
	_, _ = w.Write(slenbuf[:])
	_, _ = w.Write(s)
}

type testToken struct {
	name        CTypeString
	symbol      CTypeString
	decimals    CTypeUint8
	totalSupply CTypeUint256
}

var (
	tokenCode = []byte{
		0xd0, 0x23, 0x01,
	}
	tokenCreateFnHash = mustReadBytes4hex("4759498ac2a719c619e2c8cf8ee60af2d2407425e95d308eb208425b2a6d427a")
	tokenCreateParams = func(tt testToken) (d []byte) {
		buf := NewBuffer(nil)
		writeStringParams(buf, tt.name)
		writeStringParams(buf, tt.symbol)
		_, _ = buf.Write(tt.decimals[:])
		_, _ = buf.Write(tt.totalSupply[:])
		return buf.Bytes()
	}
	testACToken = testToken{
		name:        CTypeString("AbCoin"),
		symbol:      CTypeString("AC"),
		decimals:    NewUint8(10),
		totalSupply: NewUint256(new(big.Int).SetInt64(100)),
	}
	testAbTokenCreateParams = tokenCreateParams(testACToken)
)

func TestXvm_Create(t *testing.T) {
	st := newTestStateTree()
	vm := NewXVM(st)
	inputBuf := bytes.NewBuffer(nil)
	inputBuf.Write(tokenCode)
	inputBuf.Write(tokenCreateFnHash)
	inputBuf.Write(testAbTokenCreateParams)
	addr := common.Address{0x01}
	simpleCode := []byte("hello, world")
	if err := vm.Create(addr, simpleCode); err != nil {
		t.Fatal(err)
	}
	nonce1 := vm.stateTree.GetNonce(addr)
	c1addr := crypto.CreateAddress(addr.Hash(), nonce1)
	c1code := vm.stateTree.GetCode(c1addr)
	assert.Equal(t, c1code, simpleCode)
	if err := vm.Create(addr, inputBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	nonce2 := vm.stateTree.GetNonce(addr)
	c2addr := crypto.CreateAddress(addr.Hash(), nonce2)
	bc, err := vm.GetBuiltinContract(c2addr)
	if err != nil {
		t.Fatal(err)
	}
	tc, ok := bc.(*token)
	if !ok {
		t.Fatalf("cover token contract err")
	}
	gotobj := testToken{
		name:        tc.GetName(),
		symbol:      tc.GetSymbol(),
		decimals:    tc.GetDecimals(),
		totalSupply: tc.GetTotalSupply(),
	}
	assert.Equal(t, gotobj, testACToken)
}

func TestXvm_Run(t *testing.T) {

}
