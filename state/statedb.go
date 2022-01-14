package state

import (
	"math/big"
	"xfsgo/avlmerkle"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
	"xfsgo/crypto"
	"xfsgo/storage/badger"
	"xfsgo/types"
)

type StateDB struct {
	root       []byte
	treeDB     badger.IStorage
	merkleTree *avlmerkle.Tree
	objs       map[common.Address]*stateObject
}

func NewStateDB(db badger.IStorage, root []byte) *StateDB {
	st := &StateDB{
		root:   root,
		treeDB: db,
		objs:   make(map[common.Address]*stateObject),
	}
	st.merkleTree = avlmerkle.NewTree(st.treeDB, root)
	return st
}
func NewStateTreeN(db badger.IStorage, root []byte) (*StateDB, error) {
	var err error
	st := &StateDB{
		root:   root,
		treeDB: db,
		objs:   make(map[common.Address]*stateObject),
	}
	st.merkleTree, err = avlmerkle.NewTreeN(st.treeDB, root)
	return st, err
}
func (st *StateDB) HashAccount(addr common.Address) bool {
	return st.GetStateObj(addr) != nil
}

func (st *StateDB) GetBalance(addr common.Address) *big.Int {
	obj := st.GetStateObj(addr)
	if obj != nil {
		if obj.balance == nil {
			return zeroBigN
		}
		return obj.balance
	}
	return zeroBigN
}

func (st *StateDB) Copy() *StateDB {
	cpy := new(StateDB)
	copy(cpy.root, st.root)
	cpy.treeDB = st.treeDB
	cpy.merkleTree = st.merkleTree.Copy()
	cpy.objs = make(map[common.Address]*stateObject)
	for k, v := range st.objs {
		cpy.objs[k] = v
	}
	return cpy
}
func (st *StateDB) Set(snap *StateDB) *StateDB {
	st.root = snap.root
	st.treeDB = snap.treeDB
	st.merkleTree = snap.merkleTree
	st.objs = snap.objs
	return st
}

func (st *StateDB) GetStateRoot(addr common.Address) common.Hash {
	obj := st.GetOrNewStateObj(addr)
	if obj != nil {
		return obj.GetStateRoot()
	}

	return common.Hash{}
}

func (st *StateDB) AddBalance(addr common.Address, val *big.Int) {
	obj := st.GetOrNewStateObj(addr)
	if obj != nil {
		obj.AddBalance(val)
	}
}
func (st *StateDB) GetNonce(addr common.Address) uint64 {
	obj := st.GetStateObj(addr)
	if obj != nil {
		return obj.nonce
	}
	return 0
}

// SubBalance subtracts amount from the account associated with addr.
func (s *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (s *StateDB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (s *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (s *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (s *StateDB) SetState(addr common.Address, key, value common.Hash) {
	// stateObject := s.GetOrNewStateObj(addr)
	// if stateObject != nil {
	// 	stateObject.SetState(s.db, key, value)
	// }
}

func (st *StateDB) AddNonce(addr common.Address, val uint64) {
	obj := st.GetOrNewStateObj(addr)
	if obj != nil {
		obj.AddNonce(val)
	}
}

func (st *StateDB) GetStateObj(addr common.Address) *stateObject {
	if st.objs[addr] != nil {
		return st.objs[addr]
	}
	hash := ahash.SHA256(addr.Bytes())
	if val, has := st.merkleTree.Get(hash); has {
		obj := &stateObject{}
		if err := rawencode.Decode(val, obj); err != nil {
			return nil
		}
		obj.merkleTree = st.merkleTree
		st.objs[addr] = obj
		return obj
	}
	return nil
}

func (st *StateDB) newStateObj(address common.Address) *stateObject {
	obj := NewStateObj(address, st.merkleTree, st.treeDB)
	st.objs[obj.address] = obj
	return obj
}

func (st *StateDB) CreateAccount(addr common.Address) {
	old := st.GetStateObj(addr)
	add := st.newStateObj(addr)
	if old != nil {
		add.balance = old.balance
	}
}

func (st *StateDB) GetOrNewStateObj(addr common.Address) *stateObject {
	stateObj := st.GetStateObj(addr)
	if stateObj == nil {
		stateObj = st.newStateObj(addr)
	}
	return stateObj
}

func (st *StateDB) Root() []byte {
	return st.merkleTree.Checksum()
}

func (st *StateDB) RootHex() string {
	return st.merkleTree.ChecksumHex()
}

func (st *StateDB) UpdateAll() {
	for _, v := range st.objs {
		v.Update()
	}
}

func (st *StateDB) Commit() error {
	return st.merkleTree.Commit()
}

// AddAddressToAccessList adds the given address to the access list
func (s *StateDB) AddAddressToAccessList(addr common.Address) {
	// if s.accessList.AddAddress(addr) {
	// 	s.journal.append(accessListAddAccountChange{&addr})
	// }
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (s *StateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
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
func (s *StateDB) AddressInAccessList(addr common.Address) bool {
	// return s.accessList.ContainsAddress(addr)
	return true
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (s *StateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	// return s.accessList.Contains(addr, slot)
	return true, true
}

func (s *StateDB) AddLog(*types.Log) {

}

// AddPreimage records a SHA3 preimage seen by the VM.
func (s *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	// if _, ok := s.preimages[hash]; !ok {
	// 	s.journal.append(addPreimageChange{hash: hash})
	// 	pi := make([]byte, len(preimage))
	// 	copy(pi, preimage)
	// 	s.preimages[hash] = pi
	// }
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (s *StateDB) Preimages() map[common.Hash][]byte {
	// return s.preimages
	return nil
}

// AddRefund adds gas to the refund counter
func (s *StateDB) AddRefund(gas uint64) {
	// s.journal.append(refundChange{prev: s.refund})
	// s.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (s *StateDB) SubRefund(gas uint64) {
	// s.journal.append(refundChange{prev: s.refund})
	// if gas > s.refund {
	// 	panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, s.refund))
	// }
	// s.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (s *StateDB) Exist(addr common.Address) bool {
	// return s.getStateObject(addr) != nil
	return true
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (s *StateDB) Empty(addr common.Address) bool {
	// so := s.getStateObject(addr)
	// return so == nil || so.empty()
	return true
}

func (db *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error {
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

func (s *StateDB) GetExtra(addr common.Address) []byte {
	stateObject := s.GetOrNewStateObj(addr)
	if stateObject != nil {
		return stateObject.GetExtra()
	}

	return nil
}

func (s *StateDB) GetCode(addr common.Address) []byte {
	stateObject := s.GetStateObj(addr)
	if stateObject != nil {
		return stateObject.GetCode()
	}
	return nil
}

func (s *StateDB) GetCodeSize(addr common.Address) int {
	// stateObject := s.getStateObject(addr)
	// if stateObject != nil {
	// 	return stateObject.CodeSize(s.db)
	// }
	return 0
}

func (s *StateDB) GetCodeHash(addr common.Address) common.Hash {
	// stateObject := s.getStateObject(addr)
	// if stateObject == nil {
	// 	return common.Hash{}
	// }
	// return common.BytesToHash(stateObject.CodeHash())

	return common.Hash{}
}

// GetState retrieves a value from the given account's storage trie.
func (s *StateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	// stateObject := s.getStateObject(addr)
	// if stateObject != nil {
	// 	return stateObject.GetState(s.db, hash)
	// }
	return common.Hash{}
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (s *StateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	// stateObject := s.getStateObject(addr)
	// if stateObject != nil {
	// 	return stateObject.GetCommittedState(s.db, hash)
	// }
	return common.Hash{}
}

// GetRefund returns the current value of the refund counter.
func (s *StateDB) GetRefund() uint64 {
	// return s.refund
	return 0
}

func (s *StateDB) HasSuicided(common.Address) bool {
	return true
}

// Snapshot returns an identifier for the current revision of the state.
func (s *StateDB) Snapshot() int {
	// id := s.nextRevisionId
	// s.nextRevisionId++
	// s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	// return id
	return 0
}

func (s *StateDB) RevertToSnapshot(int) {

}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (s *StateDB) Suicide(addr common.Address) bool {
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
