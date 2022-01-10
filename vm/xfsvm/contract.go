package xfsvm

import (
	"xfsgo/common"
	"xfsgo/core"
)

type ContractHelper interface {
	SetStateTree(st core.StateTree)
	GetStateTree() (st core.StateTree)
	GetAddress() (addr common.Address)
	SetAddress(addr common.Address)
}
type BuiltinContract interface {
	ContractHelper
	BuiltinId() (id uint8)
}

type absBuiltinContract struct {
	st   core.StateTree
	addr common.Address
}

func StdBuiltinContract() *absBuiltinContract {
	return &absBuiltinContract{}
}

func (abs *absBuiltinContract) SetAddress(addr common.Address) {
	abs.addr = addr
}

func (abs *absBuiltinContract) GetAddress() (addr common.Address) {
	addr = abs.addr
	return
}

func (abs *absBuiltinContract) SetStateTree(st core.StateTree) {
	abs.st = st
	return
}

func (abs *absBuiltinContract) GetStateTree() (st core.StateTree) {
	st = abs.st
	return
}

func (abs *absBuiltinContract) BuiltinId() (id uint8) {
	return
}

func (abs *absBuiltinContract) Create(func()) (err error) {
	return
}
