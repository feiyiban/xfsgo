package xfsvm

import (
	"bytes"
	"errors"
	"io"
)

const (
	_ = uint8(iota)
	OpLoad
	OpStore
	OpPush
	OpPop
	OpAdd
)

type OpNum [8]byte

var (
	errStackOverflow = errors.New("stack overflow")
)

type vmstack struct {
	list []OpNum
}

func (vstack *vmstack) push(data OpNum) {
	vstack.list = append(vstack.list, data)
}

func (vstack *vmstack) pop() (OpNum, error) {
	if len(vstack.list) == 0 {
		return OpNum{}, errStackOverflow
	}
	data := vstack.list[len(vstack.list)-1]
	vstack.list = vstack.list[0 : len(vstack.list)-1]
	return data, nil
}

type vmc struct {
	stack *vmstack
}

func NewVMC() *vmc {
	return &vmc{
		stack: new(vmstack),
	}
}

func readData(reader io.Reader) (OpNum, error) {
	var num OpNum
	n, err := reader.Read(num[:])
	if err != nil && err == io.EOF {
		return OpNum{}, err
	}
	if n < len(OpNum{}) {
		return OpNum{}, errors.New("get data err")
	}
	return num, nil
}
func (vm *vmc) Exec(s []byte) error {
	reader := bytes.NewReader(s)
out:
	for {
		b, err := reader.ReadByte()
		if err != nil && err == io.EOF {
			break out
		}
		switch b {
		case OpLoad:
		case OpStore:
		case OpPush:
			data, err := readData(reader)
			if err != nil {
				return err
			}
			vm.stack.push(data)
		case OpPop:
			//vm.stack.pop()
		case OpAdd:
			//a := vm.stack.popUint64()
			//b := vm.stack.popUint64()
			//vm.stack.pushUint64(a + b)
		default:
			return errors.New("unknown op_code")
		}
	}
	return nil
}

func (vm *vmc) Pop() (OpNum, error) {
	return vm.stack.pop()
}
