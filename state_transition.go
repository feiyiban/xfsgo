// Copyright 2014 The go-ethereum Authors
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

package xfsgo

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"xfsgo/common"
	"xfsgo/crypto"
	"xfsgo/params"
	"xfsgo/vm/evm"

	"github.com/sirupsen/logrus"
)

var emptyCodeHash = crypto.Keccak256Hash(nil)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	gasFeeCap  *big.Int
	gasTipCap  *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      evm.StateDB
	evm        *evm.EVM
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	To() common.Address
	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int
	Nonce() uint64
	IsFake() bool
	Data() []byte
}

// ExecutionResult includes all output after executing given evm
// message no matter the execution itself is successful or not.
type ExecutionResult struct {
	UsedGas    uint64 // Total used gas but include the refunded gas
	Err        error  // Any error encountered during the execution(listed in core/vm/errors.go)
	ReturnData []byte // Returned data from evm(function result or data supplied with revert opcode)
}

// Unwrap returns the internal evm error which allows us for further
// analysis outside.
func (result *ExecutionResult) Unwrap() error {
	return result.Err
}

// Failed returns the indicator whether the execution is successful or not
func (result *ExecutionResult) Failed() bool { return result.Err != nil }

// Return is a helper function to help caller distinguish between revert reason
// and function return. Return returns the data after execution if no error occurs.
func (result *ExecutionResult) Return() []byte {
	if result.Err != nil {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// Revert returns the concrete revert reason if the execution is aborted by `REVERT`
// opcode. Note the reason can be nil if no data supplied with revert opcode.
func (result *ExecutionResult) Revert() []byte {
	if result.Err != evm.ErrExecutionReverted {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, isContractCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if isContractCreation {
		gas = common.TxGasContractCreation
	} else {
		gas = common.TxGas.Uint64()
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		nonZeroGas := params.TxDataNonZeroGasFrontier
		if (math.MaxUint64-gas)/nonZeroGas < nz {
			return 0, errors.New("gas uint64 overflow")
		}
		gas += nz * nonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			return 0, errors.New("gas uint64 overflow")
		}
		gas += z * params.TxDataZeroGas
	}

	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *evm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm *evm.EVM, msg Message, gp *GasPool) (*ExecutionResult, error) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || (st.msg.To() == common.Address{}) {
		return common.Address{}
	}
	return st.msg.To()
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).SetUint64(st.msg.Gas())
	mgval = mgval.Mul(mgval, st.gasPrice)
	balanceCheck := mgval
	if st.gasFeeCap != nil {
		balanceCheck = new(big.Int).SetUint64(st.msg.Gas())
		balanceCheck = balanceCheck.Mul(balanceCheck, st.gasFeeCap)
		balanceCheck.Add(balanceCheck, st.value)
	}
	if have, want := st.state.GetBalance(st.msg.From()), balanceCheck; have.Cmp(want) < 0 {
		fromAddr := st.msg.From()
		return fmt.Errorf("%v: address %v have %v want %v", "insufficient funds for gas * price + value", fromAddr.Hex(), have, want)
	}
	if err := st.gp.SubGas(new(big.Int).SetUint64(st.msg.Gas())); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	logrus.Debugf("st.msg.Gas():%v, mgval:%v,st.gasPrice:%v\n", st.msg.Gas(), mgval, st.gasPrice)
	st.state.SubBalance(st.msg.From(), mgval)
	return nil
}

func (st *StateTransition) refundGas() {
	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.state.AddBalance(st.msg.From(), remaining)

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(new(big.Int).SetUint64(st.gas))
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}

func (st *StateTransition) txPreCheck() error {
	// Only check transactions that are not fake
	if !st.msg.IsFake() {
		fromaddr := st.msg.From()
		stNonce := st.state.GetNonce(fromaddr)
		if stNonce != st.msg.Nonce() {
			return fmt.Errorf("nonce err: want=%d, got=%d", stNonce, st.msg.Nonce())
		}
	}

	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the evm execution result with following fields.
//
// - used gas:
//      total gas used (including gas being refunded)
// - returndata:
//      the returned data from evm
// - concrete execution error:
//      various **EVM** error which aborts the execution,
//      e.g. ErrOutOfGas, ErrExecutionReverted
//
// However if any consensus issue encountered, return the error directly with
// nil evm execution result.
func (st *StateTransition) TransitionDb() (*ExecutionResult, error) {
	// First check this message satisfies all consensus rules before
	// applying the message. The rules include these clauses
	//
	// 1. the nonce of the message caller is correct
	// 2. caller has enough balance to cover transaction fee(gaslimit * gasprice)
	// 3. the amount of gas required is available in the block
	// 4. the purchased gas is enough to cover intrinsic usage
	// 5. there is no overflow when calculating intrinsic gas
	// 6. caller has enough balance to cover asset transfer for **topmost** call

	// Check clauses 1-3, buy gas if everything is correct
	logrus.Debugf("st:%v\n", st)
	if err := st.txPreCheck(); err != nil {
		return nil, err
	}
	msg := st.msg
	sender := evm.AccountRef(msg.From())
	contractCreation := msg.To() == common.Address{}

	// Check clauses 4-5, subtract intrinsic gas if everything is correct
	logrus.Debugf("st.data:%v\n", st.data)
	gas, err := IntrinsicGas(st.data, contractCreation)
	if err != nil {
		return nil, err
	}
	if st.gas < gas {
		return nil, fmt.Errorf("%w: have %d, want %d", errors.New("intrinsic gas too low"), st.gas, gas)
	}
	st.gas -= gas
	logrus.Debugf("st.gas:%v, IntrinsicGas gas:%v\n", st.gas, gas)

	if msg.Value().Sign() > 0 && !st.evm.Context.CanTransfer(st.state, msg.From(), msg.Value()) {
		return nil, fmt.Errorf("%w: address %v", errors.New("insufficient funds for transfer"), msg.From())
	}

	var (
		ret   []byte
		vmerr error // vm errors do not effect consensus and are therefore not assigned to err
	)

	if contractCreation {
		logrus.Debugf("create contract: st.data:%v, gas:%v, st.value:%v\n", st.data, st.gas, st.value)
		ret, _, st.gas, vmerr = st.evm.Create(sender, st.data, st.gas, st.value)
		logrus.Debugf("create contract: ret st.data:%v, gas:%v, st.value:%v\n", st.data, st.gas, st.value)

	} else {
		// Increment the nonce for the next transaction
		logrus.Debugf("call contract: ret st.data:%v,gas:%v,st.value:%v\n", st.data, st.gas, st.value)
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = st.evm.Call(sender, st.to(), st.data, st.gas, st.value)
		logrus.Debugf("call contract: ret st.data:%v,gas:%v,st.value:%v\n", st.data, st.gas, st.value)
	}

	st.refundGas()

	logrus.Debugf("addbalance:%v\n", new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))
	st.state.AddBalance(st.evm.Context.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	logrus.Debugf("ReturnData:%v\n", ret)
	return &ExecutionResult{
		UsedGas:    st.gasUsed(),
		Err:        vmerr,
		ReturnData: ret,
	}, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(bc ChainContext, author *common.Address, gp *GasPool, statedb *StateTree, header *BlockHeader, tx *Transaction, usedGas *uint64, cfg evm.Config) (*Receipt, error) {
	msg, err := tx.AsMessage()
	if err != nil {
		return nil, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, author)
	vmenv := evm.NewEVM(blockContext, evm.TxContext{}, statedb, cfg)
	return applyTransaction(msg, bc, author, gp, statedb, new(big.Int).SetUint64(header.Height), header.HeaderHash(), tx, usedGas, vmenv)
}

func applyTransaction(msg MessageImp, bc ChainContext, author *common.Address, gp *GasPool, statedb *StateTree, blockNumber *big.Int, blockHash common.Hash, tx *Transaction, usedGas *uint64, evm *evm.EVM) (*Receipt, error) {
	// Create a new context to be used in the EVM environment.
	txContext := NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	// Apply the transaction to the current state (included in the env).
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	receipt := &Receipt{}
	if (msg.To() == common.Address{}) {
		receipt.ContractAddress = crypto.CreateAdrAddress(evm.TxContext.Origin, tx.Nonce)
	}

	*usedGas += result.UsedGas
	statedb.UpdateAll()
	receipt.Version = tx.Version
	receipt.TxHash = tx.Hash()
	receipt.Status = 1
	receipt.GasUsed = new(big.Int).SetUint64(*usedGas)
	receipt.Result = common.Bytes2Hex(result.ReturnData[:])

	return receipt, err
}
