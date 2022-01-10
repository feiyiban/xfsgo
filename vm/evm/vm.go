package evm

import "xfsgo/common"

type VM interface {
	Run(common.Address, []byte) error
	Create(common.Address, []byte) error
	Call(common.Address, []byte) error
}
type xvm struct {
}

func NewXVM() *xvm {
	return &xvm{}
}

// [offset] [bits] [desc]
// 0x0000   32     magic
// 0x0020   8      type
type codeHeader struct {
	magic uint32
	mType uint8
}

// [offset] [bits] [desc]
// 0x0000   40     header
// 0x0028   8      type
type codeFormat struct {
	header *codeHeader
	body   []byte
}

func readHeader([]byte) (error, *codeHeader) {
	return nil, nil
}

func readCodeFormat([]byte) (error, *codeFormat) {
	return nil, nil
}
func (vm *xvm) Run(addr common.Address, code []byte) error {
	//readCodeFormat(code)
	return nil
}
func (vm *xvm) Create(common.Address, []byte) error {
	return nil
}

func (vm *xvm) Call(common.Address, []byte) error {
	return nil
}
