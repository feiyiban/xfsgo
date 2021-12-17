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

package xfsgo

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"xfsgo/avlmerkle"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
	"xfsgo/crypto"
	"xfsgo/storage/badger"
	"xfsgo/types"
)

type Code []byte

//StateObj is an importment type which represents an xfs account that is being modified.
// The flow of usage is as follows:
// First, you need to obtain a StateObj object.
// Second, access and modify the balance of account through the object.
// Finally, call Commit method to write the modified merkleTree into a database.
type StateObj struct {
	merkleTree   *avlmerkle.Tree
	address      common.Address //hash of address of the account
	balance      *big.Int
	nonce        uint64
	code         []byte
	stateRoot    common.Hash
	cacheStorage map[[32]byte][]byte
	db           badger.IStorage
}

func loadBytesByMapKey(m map[string]string, key string) (data []byte, rt bool) {
	var str string
	var err error
	if str, rt = m[key]; rt {
		if data, err = hex.DecodeString(str); err != nil {
			rt = false
		}
	}
	return
}
func (so *StateObj) Decode(data []byte) error {
	r := common.StringDecodeMap(string(data))
	if r == nil {
		return nil
	}
	if address, ok := r["address"]; ok {
		so.address = common.StrB58ToAddress(address)
	}
	if balance, ok := r["balance"]; ok {
		if num, ok := new(big.Int).
			SetString(balance, 10); ok {
			so.balance = num
		}
	}
	if nonce, ok := r["nonce"]; ok {
		if num, ok := new(big.Int).
			SetString(nonce, 10); ok {
			so.nonce = num.Uint64()
		}
	}
	if extra, ok := r["code"]; ok {
		if bs, err := hex.DecodeString(extra); err == nil {
			so.code = bs
		}
	}

	if bs, ok := loadBytesByMapKey(r, "state_root"); ok {
		so.stateRoot = common.Bytes2Hash(bs)
	}
	return nil
}

func (so *StateObj) Encode() ([]byte, error) {
	objmap := map[string]string{
		"address": so.address.String(),
		"balance": so.balance.Text(10),
		"nonce":   new(big.Int).SetUint64(so.nonce).Text(10),
		"code":    hex.EncodeToString(so.code),
	}
	if so.code != nil {
		objmap["code"] = hex.EncodeToString(so.code)
	}
	if !bytes.Equal(so.stateRoot[:], common.HashZ[:]) {
		objmap["state_root"] = hex.EncodeToString(so.stateRoot[:])
	}
	enc := common.SortAndEncodeMap(objmap)
	return []byte(enc), nil
}

// NewStateObj creates an StateObj with accout address and tree
//func NewStateObj(address common.Address, tree *avlmerkle.Tree) *StateObj {
//	obj := &StateObj{
//		address:      address,
//		merkleTree:   tree,
//		cacheStorage: make(map[[32]byte][]byte),
//	}
//	return obj
//}

func NewStateObj(address common.Address, tree *avlmerkle.Tree, db badger.IStorage) *StateObj {
	obj := &StateObj{
		address:      address,
		merkleTree:   tree,
		db:           db,
		cacheStorage: make(map[[32]byte][]byte),
	}
	return obj
}

// AddBalance adds amount to StateObj's balance.
// It is used to add funds to the destination account of a transfer.
func (so *StateObj) AddBalance(val *big.Int) {
	if val == nil || val.Sign() <= 0 {
		return
	}
	oldBalance := so.balance
	if oldBalance == nil {
		oldBalance = zeroBigN
	}
	newBalance := new(big.Int).Add(oldBalance, val)
	so.SetBalance(newBalance)
}

// SubBalance removes amount from StateObj's balance.
// It is used to remove funds from the origin account of a transfer.
func (so *StateObj) SubBalance(val *big.Int) {
	if val == nil || val.Sign() <= 0 {
		return
	}
	oldBalance := so.balance
	if oldBalance == nil {
		oldBalance = zeroBigN
	}
	newBalance := oldBalance.Sub(oldBalance, val)
	so.SetBalance(newBalance)
}

func (so *StateObj) SetBalance(val *big.Int) {
	if val == nil || val.Sign() < 0 {
		return
	}
	so.balance = val
}

func (so *StateObj) GetBalance() *big.Int {
	return so.balance
}

// Returns the address of the contract/account
func (s *StateObj) Address() common.Address {
	return s.address
}

func (so *StateObj) SetNonce(nonce uint64) {
	so.nonce = nonce
}
func (so *StateObj) AddNonce(nonce uint64) {
	so.nonce += nonce
}
func (so *StateObj) SubNonce(nonce uint64) {
	so.nonce -= nonce
}
func (so *StateObj) GetNonce() uint64 {
	return so.nonce
}

func (so *StateObj) SetState(key [32]byte, value []byte) {
	so.cacheStorage[key] = value
}
func (so *StateObj) makeStateKey(key [32]byte) []byte {
	return ahash.SHA256(append(so.address[:], key[:]...))
}
func (so *StateObj) getStateTree() *avlmerkle.Tree {
	return avlmerkle.NewTree(so.db, so.stateRoot[:])
}

func (so *StateObj) GetStateValue(key [32]byte) []byte {
	if val, exists := so.cacheStorage[key]; exists {
		return val
	}
	if val, ok := so.getStateTree().Get(so.makeStateKey(key)); ok {
		return val
	}
	return nil
}

func (so *StateObj) GetData() []byte {
	return so.code
}

func (s *StateObj) SetCode(codeHash common.Hash, code []byte) {
	// prevcode := s.Code(s.db.db)
	// s.db.journal.append(codeChange{
	// 	account:  &s.address,
	// 	prevhash: s.CodeHash(),
	// 	prevcode: prevcode,
	// })
	s.setCode(codeHash, code)
}

func (so *StateObj) Update() {
	for k, v := range so.cacheStorage {
		so.getStateTree().Put(so.makeStateKey(k), v)
	}
	stateRoot := so.getStateTree().Checksum()
	so.stateRoot = common.Bytes2Hash(stateRoot)
	objRaw, _ := rawencode.Encode(so)
	hash := ahash.SHA256(so.address[:])
	so.merkleTree.Put(hash, objRaw)

}

func (s *StateObj) setCode(codeHash common.Hash, code []byte) {
	s.code = code
	// s.data.CodeHash = codeHash[:]
	// s.dirtyCode = true
}

// Code returns the contract code associated with this object, if any.
func (s *StateObj) Code(treeDB badger.IStorage) []byte {

	return s.code

	// code, err := treeDB.ContractCode(s.addrHash, common.BytesToHash(s.CodeHash()))
	// if err != nil {
	// 	s.setError(fmt.Errorf("can't load code hash %x: %v", s.CodeHash(), err))
	// }
	// s.code = code
}

type StateTree struct {
	root       []byte
	treeDB     badger.IStorage
	merkleTree *avlmerkle.Tree
	objs       map[common.Address]*StateObj
}

func NewStateTree(db badger.IStorage, root []byte) *StateTree {
	st := &StateTree{
		root:   root,
		treeDB: db,
		objs:   make(map[common.Address]*StateObj),
	}
	st.merkleTree = avlmerkle.NewTree(st.treeDB, root)
	return st
}
func NewStateTreeN(db badger.IStorage, root []byte) (*StateTree, error) {
	var err error
	st := &StateTree{
		root:   root,
		treeDB: db,
		objs:   make(map[common.Address]*StateObj),
	}
	st.merkleTree, err = avlmerkle.NewTreeN(st.treeDB, root)
	return st, err
}
func (st *StateTree) HashAccount(addr common.Address) bool {
	return st.GetStateObj(addr) != nil
}

func (st *StateTree) GetBalance(addr common.Address) *big.Int {
	obj := st.GetStateObj(addr)
	if obj != nil {
		if obj.balance == nil {
			return zeroBigN
		}
		return obj.balance
	}
	return zeroBigN
}

func (st *StateTree) Copy() *StateTree {
	cpy := new(StateTree)
	copy(cpy.root, st.root)
	cpy.treeDB = st.treeDB
	cpy.merkleTree = st.merkleTree.Copy()
	cpy.objs = make(map[common.Address]*StateObj)
	for k, v := range st.objs {
		cpy.objs[k] = v
	}
	return cpy
}
func (st *StateTree) Set(snap *StateTree) *StateTree {
	st.root = snap.root
	st.treeDB = snap.treeDB
	st.merkleTree = snap.merkleTree
	st.objs = snap.objs
	return st
}

func (st *StateTree) AddBalance(addr common.Address, val *big.Int) {
	obj := st.GetOrNewStateObj(addr)
	if obj != nil {
		obj.AddBalance(val)
	}
}
func (st *StateTree) GetNonce(addr common.Address) uint64 {
	obj := st.GetStateObj(addr)
	if obj != nil {
		return obj.nonce
	}
	return 0
}

// SubBalance subtracts amount from the account associated with addr.
func (s *StateTree) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (s *StateTree) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (s *StateTree) SetNonce(addr common.Address, nonce uint64) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (s *StateTree) SetCode(addr common.Address, code []byte) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (s *StateTree) SetState(addr common.Address, key, value common.Hash) {
	// stateObject := s.GetOrNewStateObj(addr)
	// if stateObject != nil {
	// 	stateObject.SetState(s.db, key, value)
	// }
}

func (st *StateTree) AddNonce(addr common.Address, val uint64) {
	obj := st.GetOrNewStateObj(addr)
	if obj != nil {
		obj.AddNonce(val)
	}
}

func (st *StateTree) GetStateObj(addr common.Address) *StateObj {
	if st.objs[addr] != nil {
		return st.objs[addr]
	}
	hash := ahash.SHA256(addr.Bytes())
	if val, has := st.merkleTree.Get(hash); has {
		obj := &StateObj{}
		if err := rawencode.Decode(val, obj); err != nil {
			return nil
		}
		obj.merkleTree = st.merkleTree
		st.objs[addr] = obj
		return obj
	}
	return nil
}

func (st *StateTree) newStateObj(address common.Address) *StateObj {
	obj := NewStateObj(address, st.merkleTree, st.treeDB)
	st.objs[obj.address] = obj
	return obj
}

func (st *StateTree) CreateAccount(addr common.Address) {
	old := st.GetStateObj(addr)
	add := st.newStateObj(addr)
	if old != nil {
		add.balance = old.balance
	}
}

func (st *StateTree) GetOrNewStateObj(addr common.Address) *StateObj {
	stateObj := st.GetStateObj(addr)
	if stateObj == nil {
		stateObj = st.newStateObj(addr)
	}
	return stateObj
}

func (st *StateTree) Root() []byte {
	return st.merkleTree.Checksum()
}

func (st *StateTree) RootHex() string {
	return st.merkleTree.ChecksumHex()
}

func (st *StateTree) UpdateAll() {
	for _, v := range st.objs {
		v.Update()
	}
}

func (st *StateTree) Commit() error {
	return st.merkleTree.Commit()
}

// AddAddressToAccessList adds the given address to the access list
func (s *StateTree) AddAddressToAccessList(addr common.Address) {
	// if s.accessList.AddAddress(addr) {
	// 	s.journal.append(accessListAddAccountChange{&addr})
	// }
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (s *StateTree) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	// addrMod, slotMod := s.accessList.AddSlot(addr, slot)
	// if addrMod {
	// 	// In practice, this should not happen, since there is no way to enter the
	// 	// scope of 'address' without having the 'address' become already added
	// 	// to the access list (via call-variant, create, etc).
	// 	// Better safe than sorry, though
	// 	s.journal.append(accessListAddAccountChange{&addr})
	// }
	// if slotMod {
	// 	s.journal.append(accessListAddSlotChange{
	// 		address: &addr,
	// 		slot:    &slot,
	// 	})
	// }
}

// AddressInAccessList returns true if the given address is in the access list.
func (s *StateTree) AddressInAccessList(addr common.Address) bool {
	// return s.accessList.ContainsAddress(addr)
	return true
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (s *StateTree) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	// return s.accessList.Contains(addr, slot)
	return true, true
}

func (s *StateTree) AddLog(*types.Log) {

}

// AddPreimage records a SHA3 preimage seen by the VM.
func (s *StateTree) AddPreimage(hash common.Hash, preimage []byte) {
	// if _, ok := s.preimages[hash]; !ok {
	// 	s.journal.append(addPreimageChange{hash: hash})
	// 	pi := make([]byte, len(preimage))
	// 	copy(pi, preimage)
	// 	s.preimages[hash] = pi
	// }
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (s *StateTree) Preimages() map[common.Hash][]byte {
	// return s.preimages
	return nil
}

// AddRefund adds gas to the refund counter
func (s *StateTree) AddRefund(gas uint64) {
	// s.journal.append(refundChange{prev: s.refund})
	// s.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (s *StateTree) SubRefund(gas uint64) {
	// s.journal.append(refundChange{prev: s.refund})
	// if gas > s.refund {
	// 	panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, s.refund))
	// }
	// s.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (s *StateTree) Exist(addr common.Address) bool {
	// return s.getStateObject(addr) != nil
	return true
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (s *StateTree) Empty(addr common.Address) bool {
	// so := s.getStateObject(addr)
	// return so == nil || so.empty()
	return true
}

func (db *StateTree) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error {
	// so := db.getStateObject(addr)
	// if so == nil {
	// 	return nil
	// }
	// it := trie.NewIterator(so.getTrie(db.db).NodeIterator(nil))

	// for it.Next() {
	// 	key := common.BytesToHash(db.trie.GetKey(it.Key))
	// 	if value, dirty := so.dirtyStorage[key]; dirty {
	// 		if !cb(key, value) {
	// 			return nil
	// 		}
	// 		continue
	// 	}

	// 	if len(it.Value) > 0 {
	// 		_, content, _, err := rlp.Split(it.Value)
	// 		if err != nil {
	// 			return err
	// 		}
	// 		if !cb(key, common.BytesToHash(content)) {
	// 			return nil
	// 		}
	// 	}
	// }
	return nil
}

func (s *StateTree) GetCode(addr common.Address) []byte {
	stateObject := s.GetStateObj(addr)
	if stateObject != nil {
		return stateObject.Code(s.treeDB)
	}
	return nil
}

func (s *StateTree) GetCodeSize(addr common.Address) int {
	// stateObject := s.getStateObject(addr)
	// if stateObject != nil {
	// 	return stateObject.CodeSize(s.db)
	// }
	return 0
}

func (s *StateTree) GetCodeHash(addr common.Address) common.Hash {
	// stateObject := s.getStateObject(addr)
	// if stateObject == nil {
	// 	return common.Hash{}
	// }
	// return common.BytesToHash(stateObject.CodeHash())

	return common.Hash{}
}

// GetState retrieves a value from the given account's storage trie.
func (s *StateTree) GetState(addr common.Address, hash common.Hash) common.Hash {
	// stateObject := s.getStateObject(addr)
	// if stateObject != nil {
	// 	return stateObject.GetState(s.db, hash)
	// }
	return common.Hash{}
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (s *StateTree) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	// stateObject := s.getStateObject(addr)
	// if stateObject != nil {
	// 	return stateObject.GetCommittedState(s.db, hash)
	// }
	return common.Hash{}
}

// GetRefund returns the current value of the refund counter.
func (s *StateTree) GetRefund() uint64 {
	// return s.refund
	return 0
}

func (s *StateTree) HasSuicided(common.Address) bool {
	return true
}

// Snapshot returns an identifier for the current revision of the state.
func (s *StateTree) Snapshot() int {
	// id := s.nextRevisionId
	// s.nextRevisionId++
	// s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	// return id
	return 0
}

func (s *StateTree) RevertToSnapshot(int) {

}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (s *StateTree) Suicide(addr common.Address) bool {
	// stateObject := s.getStateObject(addr)
	// if stateObject == nil {
	// 	return false
	// }
	// s.journal.append(suicideChange{
	// 	account:     &addr,
	// 	prev:        stateObject.suicided,
	// 	prevbalance: new(big.Int).Set(stateObject.Balance()),
	// })
	// stateObject.markSuicided()
	// stateObject.data.Balance = new(big.Int)

	return true
}
