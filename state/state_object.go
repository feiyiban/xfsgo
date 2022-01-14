package state

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"xfsgo/avlmerkle"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
	"xfsgo/storage/badger"
)

var zeroBigN = new(big.Int).SetInt64(0)

//StateObj is an importment type which represents an xfs account that is being modified.
// The flow of usage is as follows:
// First, you need to obtain a StateObj object.
// Second, access and modify the balance of account through the object.
// Finally, call Commit method to write the modified merkleTree into a database.
type stateObject struct {
	merkleTree   *avlmerkle.Tree
	address      common.Address //hash of address of the account
	balance      *big.Int
	nonce        uint64
	extra        []byte
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
func (so *stateObject) Decode(data []byte) error {
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

func (so *stateObject) Encode() ([]byte, error) {
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

func NewStateObj(address common.Address, tree *avlmerkle.Tree, db badger.IStorage) *stateObject {
	obj := &stateObject{
		address:      address,
		merkleTree:   tree,
		db:           db,
		cacheStorage: make(map[[32]byte][]byte),
	}
	return obj
}

// AddBalance adds amount to StateObj's balance.
// It is used to add funds to the destination account of a transfer.
func (so *stateObject) AddBalance(val *big.Int) {
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
func (so *stateObject) SubBalance(val *big.Int) {
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

func (so *stateObject) SetBalance(val *big.Int) {
	if val == nil || val.Sign() < 0 {
		return
	}
	so.balance = val
}

func (so *stateObject) GetBalance() *big.Int {
	return so.balance
}

// Returns the address of the contract/account
func (s *stateObject) Address() common.Address {
	return s.address
}

func (so *stateObject) SetNonce(nonce uint64) {
	so.nonce = nonce
}
func (so *stateObject) AddNonce(nonce uint64) {
	so.nonce += nonce
}
func (so *stateObject) SubNonce(nonce uint64) {
	so.nonce -= nonce
}
func (so *stateObject) GetNonce() uint64 {
	return so.nonce
}

func (so *stateObject) SetState(key [32]byte, value []byte) {
	so.cacheStorage[key] = value
}
func (so *stateObject) makeStateKey(key [32]byte) []byte {
	return ahash.SHA256(append(so.address[:], key[:]...))
}
func (so *stateObject) getStateTree() *avlmerkle.Tree {
	return avlmerkle.NewTree(so.db, so.stateRoot[:])
}

func (so *stateObject) GetStateValue(key [32]byte) []byte {
	if val, exists := so.cacheStorage[key]; exists {
		return val
	}
	if val, ok := so.getStateTree().Get(so.makeStateKey(key)); ok {
		return val
	}
	return nil
}

func (so *stateObject) GetExtra() []byte {
	return so.extra
}

func (so *stateObject) GetCode() []byte {
	return so.code
}

func (s *stateObject) SetCode(codeHash common.Hash, code []byte) {
	s.setCode(codeHash, code)
}

func (so *stateObject) Update() {
	for k, v := range so.cacheStorage {
		so.getStateTree().Put(so.makeStateKey(k), v)
	}
	stateRoot := so.getStateTree().Checksum()
	so.stateRoot = common.Bytes2Hash(stateRoot)
	objRaw, _ := rawencode.Encode(so)
	hash := ahash.SHA256(so.address[:])
	so.merkleTree.Put(hash, objRaw)

}

func (s *stateObject) setCode(codeHash common.Hash, code []byte) {
	s.code = code
	// s.data.CodeHash = codeHash[:]
	// s.dirtyCode = true
}

func (so *stateObject) GetStateRoot() common.Hash {
	return so.stateRoot
}
