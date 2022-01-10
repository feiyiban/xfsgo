package core

import (
	"math/big"
	"xfsgo/common"
)

type StateTree interface {
	GetNonce(common.Address) uint64
	AddNonce(addr common.Address, val uint64)
	GetBalance(common.Address) *big.Int
	GetCode(common.Address) []byte
	SetState(common.Address, [32]byte, []byte)
	GetStateValue(common.Address, [32]byte) []byte
	SetCode(addr common.Address, code []byte)
}
