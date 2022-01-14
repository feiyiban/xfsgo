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

package state

import (
	"sync"
	"xfsgo/common"
)

type account struct {
	stateObject *stateObject
	nstart      uint64
	nonces      []bool
}

type ManagedState struct {
	*StateDB
	mu       sync.RWMutex
	accounts map[common.Address]*account
}

func NewManageState(tree *StateDB) *ManagedState {

	return &ManagedState{
		StateDB:  tree.Copy(),
		accounts: make(map[common.Address]*account),
	}
}

func (ms *ManagedState) SetState(stateTree *StateDB) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.StateDB = stateTree
}
func (ms *ManagedState) hasAccount(addr common.Address) bool {
	_, ok := ms.accounts[addr]
	return ok
}

func (ms *ManagedState) HasAccount(addr common.Address) bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.hasAccount(addr)
}

func (ms *ManagedState) RemoveNonce(addr common.Address, n uint64) {
	if ms.hasAccount(addr) {
		ms.mu.Lock()
		defer ms.mu.Unlock()

		account := ms.getAccount(addr)
		if n-account.nstart <= uint64(len(account.nonces)) {
			reslice := make([]bool, n-account.nstart)
			copy(reslice, account.nonces[:n-account.nstart])
			account.nonces = reslice
		}
	}
}

func (ms *ManagedState) NewNonce(addr common.Address) uint64 {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	account := ms.getAccount(addr)
	for i, nonce := range account.nonces {
		if !nonce {
			return account.nstart + uint64(i)
		}
	}
	account.nonces = append(account.nonces, true)

	return uint64(len(account.nonces)-1) + account.nstart
}
func (ms *ManagedState) GetNonce(addr common.Address) uint64 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.hasAccount(addr) {
		account := ms.getAccount(addr)
		return uint64(len(account.nonces)) + account.nstart
	} else {
		return ms.StateDB.GetNonce(addr)
	}
}

func (ms *ManagedState) getAccount(addr common.Address) *account {
	if account, ok := ms.accounts[addr]; !ok {
		so := ms.GetOrNewStateObj(addr)
		ms.accounts[addr] = newAccount(so)
	} else {
		// Always make sure the state account nonce isn't actually higher
		// than the tracked one.
		so := ms.StateDB.GetStateObj(addr)
		if so != nil && uint64(len(account.nonces))+account.nstart < so.nonce {
			ms.accounts[addr] = newAccount(so)
		}

	}

	return ms.accounts[addr]
}
func (ms *ManagedState) SetNonce(addr common.Address, nonce uint64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	so := ms.GetOrNewStateObj(addr)
	so.SetNonce(nonce)

	ms.accounts[addr] = newAccount(so)
}

func newAccount(so *stateObject) *account {
	return &account{so, so.nonce, nil}
}
