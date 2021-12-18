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
	"container/heap"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/crypto"

	"github.com/sirupsen/logrus"
)

// var defaultGasPrice = new(big.Int).SetUint64(1)    //150000000000

// transaction type
type TxType uint8

const (
	Binary TxType = iota
	LoginCandidate
	LogoutCandidate
	Delegate
	UnDelegate
)

// Transaction type.
type Transaction struct {
	Version   uint32         `json:"version"`
	To        common.Address `json:"to"`
	GasPrice  *big.Int       `json:"gas_price"`
	GasLimit  *big.Int       `json:"gas_limit"`
	Data      []byte         `json:"data"`
	Nonce     uint64         `json:"nonce"`
	Value     *big.Int       `json:"value"`
	Signature []byte         `json:"signature"`
	Type      TxType         `json:"type"`
}

type StdTransaction struct {
	Version   uint32         `json:"version"`
	To        common.Address `json:"to"`
	GasPrice  *big.Int       `json:"gas_price"`
	GasLimit  *big.Int       `json:"gas_limit"`
	Data      []byte         `json:"data"`
	Nonce     uint64         `json:"nonce"`
	Value     *big.Int       `json:"value"`
	Signature []byte         `json:"signature"`
	Type      TxType         `json:"type"`
}

func NewTransaction(to common.Address, gasLimit, gasPrice *big.Int, value *big.Int) *Transaction {
	result := &Transaction{
		Version:  version0,
		To:       to,
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		Value:    value,
	}
	return result
}

func NewTransactionByStd(tx *StdTransaction) *Transaction {
	result := &Transaction{
		Version:   version0,
		To:        common.Address{},
		GasPrice:  new(big.Int),
		GasLimit:  new(big.Int),
		Data:      tx.Data,
		Nonce:     tx.Nonce,
		Value:     new(big.Int),
		Signature: tx.Signature,
		Type:      tx.Type,
	}
	if tx.Version != result.Version {
		result.Version = tx.Version
	}
	if !bytes.Equal(tx.To[:], common.ZeroAddr[:]) {
		result.To = tx.To
	}
	if tx.GasPrice != nil {
		result.GasPrice.Set(tx.GasPrice)
	}
	if tx.GasLimit != nil {
		result.GasLimit.Set(tx.GasLimit)
	}
	if tx.Value != nil {
		result.Value.Set(tx.Value)
	}
	return result
}
func NewTransactionByStdAndSign(tx *StdTransaction, key *ecdsa.PrivateKey) *Transaction {
	result := &Transaction{
		Version:   version0,
		To:        common.Address{},
		GasPrice:  new(big.Int),
		GasLimit:  new(big.Int),
		Data:      tx.Data,
		Nonce:     tx.Nonce,
		Value:     new(big.Int),
		Signature: tx.Signature,
	}
	if tx.Version != result.Version {
		result.Version = tx.Version
	}
	if !bytes.Equal(tx.To[:], common.ZeroAddr[:]) {
		result.To = tx.To
	}
	if tx.GasPrice != nil {
		result.GasPrice.Set(tx.GasPrice)
	}
	if tx.GasLimit != nil {
		result.GasLimit.Set(tx.GasLimit)
	}
	if tx.Value != nil {
		result.Value.Set(tx.Value)
	}
	if err := result.SignWithPrivateKey(key); err != nil {
		panic(err)
	}
	return result
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b []*Transaction) []*Transaction {
	keep := make([]*Transaction, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}
	return keep
}

func (t *Transaction) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *Transaction) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

func (t *Transaction) Hash() common.Hash {
	data := ""
	if t.Data != nil && len(t.Data) > 0 {
		data = "0x" + hex.EncodeToString(t.Data)
	}
	tmp := map[string]string{
		"version":   strconv.FormatInt(int64(t.Version), 10),
		"to":        t.To.String(),
		"gas_price": t.GasPrice.Text(10),
		"gas_limit": t.GasLimit.Text(10),
		"data":      data,
		"nonce":     strconv.Itoa(int(t.Nonce)),
		"value":     t.Value.Text(10),
		"signature": hex.EncodeToString(t.Signature),
	}
	enc := sortAndEncodeMap(tmp)
	if enc == "" {
		return common.Hash{}
	}
	return common.Bytes2Hash(ahash.SHA256([]byte(enc)))
}

func (t *Transaction) From() common.Address {
	pub, err := t.publicKey()
	if err != nil {
		return common.Address{}
	}
	addr := crypto.DefaultPubKey2Addr(*pub)
	return addr
}

func sortAndEncodeMap(data map[string]string) string {
	mapkeys := make([]string, 0)
	for k := range data {
		mapkeys = append(mapkeys, k)
	}
	sort.Strings(mapkeys)
	strbuf := ""
	for i, key := range mapkeys {
		val := data[key]
		if val == "" {
			continue
		}
		strbuf += fmt.Sprintf("%s=%s", key, val)
		if i < len(mapkeys)-1 {
			strbuf += "&"
		}
	}
	return strbuf
}

func (t *Transaction) SignHash() common.Hash {
	//nt := t.copyTrim()
	data := ""
	if t.Data != nil && len(t.Data) > 0 {
		data = "0x" + hex.EncodeToString(t.Data)
	}
	tmp := map[string]string{
		"version":   strconv.FormatInt(int64(t.Version), 10),
		"to":        t.To.String(),
		"gas_price": t.GasPrice.Text(10),
		"gas_limit": t.GasLimit.Text(10),
		"data":      data,
		"nonce":     strconv.Itoa(int(t.Nonce)),
		"value":     t.Value.Text(10),
	}
	enc := sortAndEncodeMap(tmp)
	if enc == "" {
		return common.Hash{}
	}
	return common.Bytes2Hash(ahash.SHA256([]byte(enc)))
}

func (t *Transaction) Cost() *big.Int {
	i := big.NewInt(0)
	i.Mul(t.GasLimit, t.GasPrice)
	i.Add(i, t.Value)
	return i
}
func (t *Transaction) SignWithPrivateKey(key *ecdsa.PrivateKey) error {
	hash := t.SignHash()
	sig, err := crypto.ECDSASign(hash.Bytes(), key)
	if err != nil {
		return err
	}
	t.Signature = sig
	return nil
}

func (t *Transaction) VerifySignature() bool {
	if _, err := t.publicKey(); err != nil {
		logrus.Warnf("Failed verify signature: %s", err)
		return false
	}
	return true
}

func (t *Transaction) publicKey() (*ecdsa.PublicKey, error) {
	hash := t.SignHash()
	return crypto.SigToPub(hash[:], t.Signature)
}

//FromAddr checks the validation of public key from the signature in the transaction.
//if right, returns the address calculated by this public key.
func (t *Transaction) FromAddr() (common.Address, error) {
	pub, err := t.publicKey()
	if err != nil {
		logrus.Warnf("Failed parse from addr by signature: %s", err)
		return common.Address{}, err
	}
	addr := crypto.DefaultPubKey2Addr(*pub)
	return addr, nil
}

func (t *Transaction) String() string {
	jsondata, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return string(jsondata)
}

type Transactions []*Transaction
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].Nonce < s[j].Nonce }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].GasPrice.Cmp(s[j].GasPrice) > 0 }
func (s TxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

func (s *TxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

func SortByPriceAndNonce(txs []*Transaction) {
	// Separate the transactions by account and sort by nonce
	byNonce := make(map[common.Address][]*Transaction)
	for _, tx := range txs {
		acc, _ := tx.FromAddr() // we only sort valid txs so this cannot fail
		byNonce[acc] = append(byNonce[acc], tx)
	}
	for _, accTxs := range byNonce {
		sort.Sort(TxByNonce(accTxs))
	}
	// Initialize a price based heap with the head transactions
	byPrice := make(TxByPrice, 0, len(byNonce))
	for acc, accTxs := range byNonce {
		byPrice = append(byPrice, accTxs[0])
		byNonce[acc] = accTxs[1:]
	}
	heap.Init(&byPrice)

	// Merge by replacing the best with the next from the same account
	txs = txs[:0]
	for len(byPrice) > 0 {
		// Retrieve the next best transaction by price
		best := heap.Pop(&byPrice).(*Transaction)

		// Push in its place the next transaction from the same account
		acc, _ := best.FromAddr() // we only sort valid txs so this cannot fail
		if accTxs, ok := byNonce[acc]; ok && len(accTxs) > 0 {
			heap.Push(&byPrice, accTxs[0])
			byNonce[acc] = accTxs[1:]
		}
		// Accumulate the best priced transaction
		txs = append(txs, best)
	}
}

// MessageImp is a fully derived transaction and implements Message
type MessageImp struct {
	to       common.Address
	from     common.Address
	nonce    uint64
	amount   *big.Int
	gasLimit uint64
	gasPrice *big.Int
	isFake   bool
	data     []byte
}

func NewMessage(from common.Address, to common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, isFake bool) MessageImp {
	return MessageImp{
		from:     from,
		to:       to,
		nonce:    nonce,
		amount:   amount,
		gasLimit: gasLimit,
		gasPrice: gasPrice,
		isFake:   isFake,
		data:     data,
	}
}

// AsMessage returns the transaction as a core.Message.
func (tx *Transaction) AsMessage() (MessageImp, error) {
	msg := MessageImp{
		nonce:    tx.Nonce,
		gasLimit: tx.GasLimit.Uint64(),
		gasPrice: tx.GasPrice,
		to:       tx.To,
		amount:   tx.Value,
		data:     tx.Data,
	}
	// // If baseFee provided, set gasPrice to effectiveGasPrice.
	// if baseFee != nil {
	// 	msg.gasPrice = math.BigMin(msg.gasPrice.Add(msg.gasTipCap, baseFee), msg.gasFeeCap)
	// }
	var err error
	msg.from, err = tx.FromAddr()
	return msg, err
}

func (m MessageImp) From() common.Address { return m.from }
func (m MessageImp) To() common.Address   { return m.to }
func (m MessageImp) GasPrice() *big.Int   { return m.gasPrice }
func (m MessageImp) Value() *big.Int      { return m.amount }
func (m MessageImp) Gas() uint64          { return m.gasLimit }
func (m MessageImp) Nonce() uint64        { return m.nonce }
func (m MessageImp) Data() []byte         { return m.data }
func (m MessageImp) IsFake() bool         { return m.isFake }
