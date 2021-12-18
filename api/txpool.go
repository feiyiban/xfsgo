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

package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"xfsgo"
	"xfsgo/common"
	"xfsgo/crypto"

	"github.com/sirupsen/logrus"
)

type TxPoolHandler struct {
	TxPool *xfsgo.TxPool
}

type GetTranByHashArgs struct {
	Hash string `json:"hash"`
}

// type ModTranGasArgs struct {
// 	GasLimit string `json:"gas_limit"`
// 	GasPrice string `json:"gas_price"`
// 	Hash     string `json:"hash"`
// }

type RawTransactionArgs struct {
	Data string `json:"data"`
}

type RemoveTxHashArgs struct {
	Hash string `json:"hash"`
}
type StringRawTransaction struct {
	Version   string `json:"version"`
	To        string `json:"to"`
	Value     string `json:"value"`
	Data      string `json:"data"`
	GasLimit  string `json:"gas_limit"`
	GasPrice  string `json:"gas_price"`
	Signature string `json:"signature"`
	Nonce     string `json:"nonce"`
	Type      int    `json:"type"`
}

func (tx *TxPoolHandler) GetPending(_ EmptyArgs, resp **TransactionsResp) error {
	data := tx.TxPool.GetTransactions()
	return coverTxs2Resp(data, resp)
}

func (tx *TxPoolHandler) GetQueue(_ EmptyArgs, resp **TransactionsResp) error {
	data := tx.TxPool.GetQueues()
	return coverTxs2Resp(data, resp)
}

func (tx *TxPoolHandler) GetPendingSize(_ EmptyArgs, resp *int) error {
	data := tx.TxPool.GetTransactionsSize()
	*resp = data
	return nil
}

func (tx *TxPoolHandler) RemoveTx(args RemoveTxHashArgs, resp *string) error {
	if args.Hash == "" {
		return xfsgo.NewRPCError(-1006, "Parameter cannot be empty")
	}
	if err := common.HashCalibrator(args.Hash); err != nil {
		return xfsgo.NewRPCErrorCause(-32001, err)
	}
	txHash := common.Hex2Hash(args.Hash)
	tx.TxPool.RemoveTx(txHash)
	return nil
}

func (tx *TxPoolHandler) Clear(_ EmptyArgs, resp *string) error {
	tx.TxPool.RemoveTransactions(tx.TxPool.GetTransactions())
	return nil
}

func (tx *TxPoolHandler) GetTranByHash(args GetTranByHashArgs, resp **xfsgo.Transaction) error {
	if args.Hash == "" {
		return xfsgo.NewRPCError(-1006, "Parameter cannot be empty")
	}
	if err := common.HashCalibrator(args.Hash); err != nil {
		return xfsgo.NewRPCErrorCause(-32001, err)
	}
	tranObj := tx.TxPool.GetTransaction(args.Hash)
	if tranObj != nil {
		*resp = tranObj
	}
	return nil
}

func (tx *TxPoolHandler) SendRawTransaction(args RawTransactionArgs, resp *string) error {
	if args.Data == "" {
		return xfsgo.NewRPCError(-1006, "Parameter data cannot be empty")
	}
	//logrus.Debugf("Handle RPC request by SendRawTransaction: args.data=%s", args.Data)
	//databytes, err := urlsafeb64.Decode(args.Data)
	databytes, err := base64.StdEncoding.DecodeString(args.Data)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-32001, fmt.Errorf("failed to parse data: %s", err))
	}
	rawtx := &StringRawTransaction{}
	if err := json.Unmarshal(databytes, rawtx); err != nil {
		return xfsgo.NewRPCErrorCause(-32001, fmt.Errorf("failed to parse data: %s", err))
	}
	logrus.Debugf("Successfully parse transaction json data: %s", string(databytes))
	txdata, err := CoverTransaction(rawtx)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-32001, err)
	}
	if err := tx.TxPool.Add(txdata); err != nil {
		return xfsgo.NewRPCErrorCause(-32001, err)
	}
	txhash := txdata.Hash()
	*resp = txhash.Hex()
	return nil
}

func (t *StringRawTransaction) String() string {
	jsondata, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return string(jsondata)
}

func CoverTransaction(r *StringRawTransaction) (*xfsgo.Transaction, error) {
	version, err := strconv.ParseInt(r.Version, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %s", err)
	}
	signature := common.Hex2bytes(r.Signature)
	if signature == nil || len(signature) < 1 {
		return nil, fmt.Errorf("failed to parse signature: %s", err)
	}
	toaddr := common.ZeroAddr
	if r.To != "" {
		toaddr = common.StrB58ToAddress(r.To)
		if !crypto.VerifyAddress(toaddr) {
			return nil, fmt.Errorf("failed to verify 'to' address: %s", r.To)
		}
	} else if r.Data == "" {
		return nil, fmt.Errorf("failed to parse 'to' address")
	}
	gasprice, ok := new(big.Int).SetString(r.GasPrice, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse gasprice")
	}
	gaslimit, ok := new(big.Int).SetString(r.GasLimit, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse gasprice")
	}
	data := common.Hex2bytes(r.Data)
	nonce, err := strconv.ParseInt(r.Nonce, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse nonce: %s", err)
	}
	value, ok := new(big.Int).SetString(r.Value, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse value")
	}
	return xfsgo.NewTransactionByStd(&xfsgo.StdTransaction{
		Version:   uint32(version),
		To:        toaddr,
		GasPrice:  gasprice,
		GasLimit:  gaslimit,
		Data:      data,
		Nonce:     uint64(nonce),
		Value:     value,
		Signature: signature,
		Type:      xfsgo.TxType(r.Type),
	}), nil
}
