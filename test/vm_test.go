// Copyright 2016 The go-ethereum Authors

// This file is part of the go-ethereum library.

//

// The go-ethereum library is free software: you can redistribute it and/or modify

// it under the terms of the GNU Lesser General Public License as published by

// the Free Software Foundation, either version 3 of the License, or

// (at your option) any later version.

//

// The go-ethereum library is distributed in the hope that it will be useful,

// but WITHOUT ANY WARRANTY; without even the implied warranty of

// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the

// GNU Lesser General Public License for more details.

//

// You should have received a copy of the GNU Lesser General Public License

// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package test

import (
	"fmt"
	"math/big"
	"testing"
	"xfsgo"
	"xfsgo/common"

	"xfsgo/vm/evm"
)

var (
	testHash    = common.Hash{}
	fromAddress = common.StrB58ToAddress("bQMEGRNy9iAyfEugoBoVnHpLnuprV381H")
	toAddress   = common.Hash{} //common.StrB58ToAddress("fLmaBWVaCjwGJn1NJomQ9SzpcfPm18kre")
	amount      = big.NewInt(1)
	nonce       = uint64(0)
	gasLimit    = big.NewInt(100000)
)

type ChainContext struct{}

func (cc ChainContext) GetHeader(hash common.Hash, number uint64) *xfsgo.BlockHeader {
	return &xfsgo.BlockHeader{
		Height:        0,
		Version:       0,
		HashPrevBlock: hash,
		Timestamp:     0,
		Coinbase:      fromAddress,
		// merkle tree root hash
		StateRoot:        common.Hash{},
		TransactionsRoot: common.Hash{},
		ReceiptsRoot:     common.Hash{},
		GasLimit:         big.NewInt(100000),
		GasUsed:          big.NewInt(100000),
		// pow consensus.
		Bits:       4278190109,
		Nonce:      0,
		ExtraNonce: 0,
	}
}

// constact multiply(int,int)
// code : "0x606060405260818060106000396000f360606040526000357c010000000000000000000000000000000000000000000000000000000090048063165c4a16146039576035565b6002565b34600257605a60048080359060200190919080359060200190919050506070565b6040518082815260200191505060405180910390f35b60008183029050607b565b9291505056"
// input1 :"165c4a1600000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000004"
// input2: "165c4a1600000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002"
func TestNewEVMBlockContext(t *testing.T) {
	data := "0x606060405260818060106000396000f360606040526000357c010000000000000000000000000000000000000000000000000000000090048063165c4a16146039576035565b6002565b34600257605a60048080359060200190919080359060200190919050506070565b6040518082815260200191505060405180910390f35b60008183029050607b565b9291505056"

	cc := ChainContext{}
	ctx := xfsgo.NewEVMBlockContext(cc.GetHeader(testHash, 0), cc, &fromAddress)

	msg := xfsgo.NewMessage(fromAddress, common.Address{}, nonce, amount, 1000, big.NewInt(1), common.Hex2bytes(data), true)
	txContext := xfsgo.NewEVMTxContext(msg)

	stateDB := NewMemStorage()
	stateTree := xfsgo.NewStateTree(stateDB, nil)
	stateTree.GetOrNewStateObj(fromAddress)
	stateTree.AddBalance(fromAddress, big.NewInt(1e18))
	testBalance := stateTree.GetBalance(fromAddress)
	fmt.Println("init testBalance =", testBalance)

	logConfig := evm.LogConfig{}
	structLogger := evm.NewStructLogger(&logConfig)
	vmConfig := evm.Config{Debug: true, Tracer: structLogger /*, JumpTable: vm.NewByzantiumInstructionSet()*/}

	evm_ := evm.NewEVM(ctx, txContext, stateTree, vmConfig)
	contractRef := evm.AccountRef(fromAddress)
	fmt.Println(fromAddress, contractRef)
	contractCode, contractAddr, gasLeftover, err := evm_.Create(contractRef, common.Hex2bytes(data), stateTree.GetBalance(fromAddress).Uint64(), big.NewInt(1000000000))
	fmt.Printf("err:%v\n", err)

	fmt.Printf("-------contractAddr:%v,gasLeftover:%v\n", contractAddr.Hex(), gasLeftover)
	fmt.Printf("code_contractCode:%v\n", contractCode)
	fmt.Printf("stateTree.GetCode()_contractCode:%v\n", stateTree.GetCode(contractAddr))

	testBalance = stateTree.GetBalance(fromAddress)
	contractAddrBal := stateTree.GetBalance(contractAddr)
	fmt.Printf("after create contract, Balance=%v,contractAddrBal=%v\n", testBalance, contractAddrBal)

	input := "165c4a1600000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000004"

	outputs1, gasLeftover1, vmerr := evm_.Call(contractRef, contractAddr, common.Hex2bytes(input), stateTree.GetBalance(fromAddress).Uint64(), big.NewInt(0))

	if vmerr != nil {
		t.Fatal(vmerr)
	}
	fmt.Printf("call outputs %v\n", outputs1)
	fmt.Printf("call gasLeftover %v\n", gasLeftover1)

}
