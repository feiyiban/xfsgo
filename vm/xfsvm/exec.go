package xfsvm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/core"
)

var (
	errNotfoundCreateFn = errors.New("notfound create function")
	errNotfoundMethod   = errors.New("notfound method")
	errUnsupportedType  = errors.New("unsupported type")
)

const contractTag = "contract"
const contractStorage = "storage"

type ContractExec interface {
	Create(input []byte) (err error)
	Call(input []byte) (err error)
}

type builtinContractExec struct {
	code      []byte
	stateTree core.StateTree
	address   common.Address
	contractT reflect.Type
	resultBuf Buffer
}

type stv struct {
	reflect.StructField
	nameHash [32]byte
	val      reflect.Value
}

func (ce *builtinContractExec) goReturn(vs []reflect.Value) error {
	for i := 0; i < len(vs); i++ {
		if vs[i].IsNil() {
			continue
		}
		if err, ok := vs[i].Interface().(error); ok {
			return err
		}
		_, _ = ce.resultBuf.Write(vs[i].Bytes())
	}
	return nil
}
func (ce *builtinContractExec) buildContract() (bc BuiltinContract, err error) {
	ins := reflect.New(ce.contractT)
	bc, ok := ins.Interface().(BuiltinContract)
	if !ok {
		return nil, errors.New("")
	}
	return
}
func (ce *builtinContractExec) call(fn reflect.Method, fnv reflect.Value, input []byte) error {
	buf := NewBuffer(input)
	mType := fn.Type
	n := mType.NumIn()

	var args = make([]reflect.Value, 0)
	for i := 1; i < n; i++ {
		parameterType := mType.In(i)
		switch parameterType {
		case reflect.TypeOf(CTypeString{}):
			ssize, err := buf.ReadUint32()
			if err != nil {
				return err
			}
			s, err := buf.ReadString(int(ssize.Uint32()))
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(s))
		case reflect.TypeOf(CTypeUint8{}):
			m, err := buf.ReadUint8()
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(m))
		case reflect.TypeOf(CTypeUint256{}):
			m, err := buf.ReadUint256()
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(m))
		}
	}
	r := fnv.Call(args)
	return ce.goReturn(r)
}

func (ce *builtinContractExec) updateContractState(stvs []*stv) (err error) {
	for i := 0; i < len(stvs); i++ {
		st := stvs[i]
		fvalue := st.val
		if !fvalue.CanInterface() {
			continue
		}
		jb, err := json.Marshal(fvalue.Interface())
		if err != nil {
			return err
		}
		ce.stateTree.SetState(ce.address, st.nameHash, jb)
		//fmt.Printf("name: %s, hash: %x, type: %v, val: %s\n", st.Name, st.nameHash[:], st.Type, string(jb))
	}
	return
}
func (ce *builtinContractExec) callFn(c BuiltinContract, stvs []*stv, fn common.Hash, input []byte) (err error) {
	cv := reflect.ValueOf(c)
	findMethod := func(hash common.Hash) (reflect.Method, reflect.Value, bool) {
		for i := 0; i < ce.contractT.NumMethod(); i++ {
			sf := ce.contractT.Method(i)
			aname := sf.Name
			namehash := ahash.SHA256([]byte(aname))
			if sf.Type.Kind() == reflect.Func && bytes.Equal(hash[:], namehash) {
				mv := cv.MethodByName(aname)
				return sf, mv, true
			}
		}
		return reflect.Method{}, reflect.Value{}, false
	}
	if m, mv, ok := findMethod(fn); ok {
		if err = ce.call(m, mv, input); err != nil {
			return
		}
		if err = ce.updateContractState(stvs); err != nil {
			return
		}
		return
	}
	return errNotfoundMethod
}
func readCallMethod(r io.Reader) (m common.Hash, e error) {
	var hashdata [32]byte
	n, e := r.Read(hashdata[:])
	if e != nil {
		return common.Hash{}, e
	}
	if n != len(hashdata) {
		return common.Hash{}, errors.New("eof")
	}
	copy(m[:], hashdata[:])
	return
}
func (ce *builtinContractExec) exec(input []byte) error {
	bc, stvs, err := ce.MakeBuiltinContract()
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(input)
	fn, err := readCallMethod(buf)
	if err != nil {
		return err
	}
	return ce.callFn(bc, stvs, fn, buf.Bytes())
}
func (ce *builtinContractExec) Call(input []byte) error {
	return ce.exec(input)
}

func (ce *builtinContractExec) Create(input []byte) error {
	return ce.exec(input)
}

func (ce *builtinContractExec) findContractStorageValue(cve reflect.Value) []*stv {
	cte := ce.contractT.Elem()
	stvs := make([]*stv, 0)
	for i := 0; i < cte.NumField(); i++ {
		ctef := cte.Field(i)
		c := ctef.Tag.Get(contractTag)
		if c != contractStorage {
			continue
		}
		nameHash := ahash.SHA256Array([]byte(ctef.Name))
		fvalue := cve.FieldByName(ctef.Name)
		if !fvalue.CanInterface() {
			continue
		}
		stvs = append(stvs, &stv{
			StructField: ctef,
			nameHash:    nameHash,
			val:         fvalue,
		})
	}
	return stvs
}
func (ce *builtinContractExec) setupContract(c interface{}, stvs []*stv) (err error) {
	var buf strings.Builder
	buf.WriteString("{")
	for i := 0; i < len(stvs); i++ {
		st := stvs[i]
		data := ce.stateTree.GetStateValue(ce.address, st.nameHash)
		if data == nil {
			continue
		}
		prefix := fmt.Sprintf("\"%s\":", st.Name)
		buf.WriteString(prefix)
		buf.Write(data)
		if i < len(stvs)-1 {
			buf.WriteString(",")
		}
	}
	buf.WriteString("}")
	bs := buf.String()
	err = json.Unmarshal([]byte(bs), &c)
	return
}
func (ce *builtinContractExec) MakeBuiltinContract() (BuiltinContract, []*stv, error) {
	cv := reflect.New(ce.contractT.Elem())

	stvs := ce.findContractStorageValue(cv.Elem())
	if err := ce.setupContract(cv.Interface(), stvs); err != nil {
		return nil, nil, err
	}
	return cv.Interface().(BuiltinContract), stvs, nil
}
