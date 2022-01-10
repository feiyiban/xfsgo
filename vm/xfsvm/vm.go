package xfsvm

import (
	"encoding/binary"
	"errors"
	"reflect"
	"xfsgo/common"
	"xfsgo/core"
	"xfsgo/crypto"
)

type VM interface {
	Run(common.Address, []byte, []byte) error
	Create(common.Address, []byte) error
	Call(common.Address, []byte) error
}

const (
	magicNumberXVM = uint16(9168)
)

var (
	errUnknownMagicNumber  = errors.New("unknown magic number")
	errUnknownContractId   = errors.New("unknown contract type")
	errUnknownContractExec = errors.New("unknown contract exec")
	errInvalidContractCode = errors.New("invalid contract code")
)

type xvm struct {
	stateTree core.StateTree
	builtins  map[uint8]reflect.Type
	returnBuf Buffer
}

func NewXVM(st core.StateTree) *xvm {
	vm := &xvm{
		stateTree: st,
		builtins:  make(map[uint8]reflect.Type),
		returnBuf: NewBuffer(nil),
	}
	vm.registerBuiltinId(new(token))
	return vm
}
func (vm *xvm) newBuiltinContractExec(id uint8, address common.Address, code []byte) (*builtinContractExec, error) {
	if ct, exists := vm.builtins[id]; exists {
		return &builtinContractExec{
			contractT: ct,
			stateTree: vm.stateTree,
			address:   address,
			code:      code,
			resultBuf: NewBuffer(nil),
		}, nil
	}
	return nil, errUnknownContractId
}
func (vm *xvm) registerBuiltinId(b BuiltinContract) {
	bid := b.BuiltinId()
	if _, exists := vm.builtins[bid]; !exists {
		rt := reflect.TypeOf(b)
		vm.builtins[bid] = rt
	}
}
func readXVMCode(code []byte, input []byte) (c []byte, id uint8, err error) {
	if code == nil && input != nil {
		code = make([]byte, 3)
		copy(code[:], input[:])
	}
	if code == nil || len(code) < 3 {
		return code, 0, errInvalidContractCode
	}
	m := binary.LittleEndian.Uint16(code[:2])
	if m != magicNumberXVM {
		return code, 0, errUnknownMagicNumber
	}
	c = code
	id = code[2]
	return
}
func (vm *xvm) Run(addr common.Address, code []byte, input []byte) (err error) {
	var create = code == nil
	code, id, err := readXVMCode(code, input)
	if err != nil && create {
		vm.stateTree.AddNonce(addr, 1)
		vm.stateTree.SetCode(addr, input)
		return nil
	} else if err != nil {
		return nil
	}
	var exec ContractExec
	if id != 0 {
		if exec, err = vm.newBuiltinContractExec(id, addr, code); err != nil {
			return
		}
	}
	if exec == nil {
		return errUnknownContractExec
	}
	if create {
		var realInput = make([]byte, len(input)-3)
		copy(realInput[:], input[3:])
		if err = exec.Create(realInput); err != nil {
			return err
		}
		vm.stateTree.AddNonce(addr, 1)
		vm.stateTree.SetCode(addr, code)
		return nil
	}
	if err = exec.Call(input); err != nil {
		return err
	}
	return nil
}
func (vm *xvm) Create(addr common.Address, input []byte) error {
	nonce := vm.stateTree.GetNonce(addr)
	caddr := crypto.CreateAddress(addr.Hash(), nonce)
	if err := vm.Run(caddr, nil, input); err != nil {
		return err
	}
	return nil
}

func (vm *xvm) Call(address common.Address, input []byte) error {
	code := vm.stateTree.GetCode(address)
	if err := vm.Run(address, code, input); err != nil {
		return err
	}
	return nil
}
func (vm *xvm) GetBuiltinContract(address common.Address) (c interface{}, err error) {
	code := vm.stateTree.GetCode(address)
	code, id, err := readXVMCode(code, nil)
	if err != nil {
		return
	}
	var exec *builtinContractExec
	if exec, err = vm.newBuiltinContractExec(id, address, code); err != nil {
		return
	}
	c, _, err = exec.MakeBuiltinContract()
	return
}
