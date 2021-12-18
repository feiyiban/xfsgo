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
	"bytes"
	"encoding/hex"
	"math/big"
	"sort"
	"xfsgo"
	"xfsgo/common"
)

type WalletHandler struct {
	Wallet        *xfsgo.Wallet
	BlockChain    *xfsgo.BlockChain
	TxPendingPool *xfsgo.TxPool
}

type WalletByAddressArgs struct {
	Address string `json:"address"`
}

type WalletImportArgs struct {
	Key string `json:"key"`
}

type SetDefaultAddrArgs struct {
	Address string `json:"address"`
}

// type TransferArgs struct {
// 	To       string `json:"to"`
// 	GasLimit string `json:"gas_limit"`
// 	GasPrice string `json:"gas_price"`
// 	Value    string `json:"value"`
// }

type SetGasLimitArgs struct {
	Gas string `json:"gas"`
}

type SetGasPriceArgs struct {
	GasPrice string `json:"gas_price"`
}

func (handler *WalletHandler) Create(_ EmptyArgs, resp *string) error {
	addr, err := handler.Wallet.AddByRandom()
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	*resp = addr.B58String()
	return nil
}
func (handler *WalletHandler) CreateToken(_ EmptyArgs, resp *string) error {
	addr, err := handler.Wallet.AddByRandom()
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	*resp = addr.B58String()
	return nil
}
func (handler *WalletHandler) CreateNFT(_ EmptyArgs, resp *string) error {
	addr, err := handler.Wallet.AddByRandom()
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	*resp = addr.B58String()
	return nil
}
func (handler *WalletHandler) Del(args WalletByAddressArgs, resp *interface{}) error {
	if args.Address == "" {
		return xfsgo.NewRPCError(-1006, "del wallet address not null")
	}
	if err := common.AddrCalibrator(args.Address); err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	addr := common.StrB58ToAddress(args.Address)
	err := handler.Wallet.Remove(addr)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	return nil
}

func (handler *WalletHandler) List(_ EmptyArgs, resp *[]common.Address) error {
	data := handler.Wallet.All()
	var out Wallets
	for addr, v := range data {
		_ = v
		r, err := handler.Wallet.GetWalletNewTime(addr)
		if err != nil {
			return err
		}
		req := &Wallet{
			addr:    addr,
			newTime: int64(common.Byte2Int(r)),
		}
		out = append(out, req)
	}

	sort.Sort(out)
	result := make([]common.Address, 0)
	for i := 0; i < len(out); i++ {
		result = append(result, out[i].addr)
	}

	*resp = result
	return nil
}

func (handler *WalletHandler) GetDefaultAddress(_ EmptyArgs, resp *string) error {
	address := handler.Wallet.GetDefault()
	zero := [25]byte{0}
	if bytes.Compare(address.Bytes(), zero[:]) == common.Zero {
		return nil
	}
	*resp = address.B58String()
	return nil
}

func (handler *WalletHandler) SetDefaultAddress(args SetDefaultAddrArgs, _ **string) error {
	if args.Address == "" {
		return xfsgo.NewRPCError(-1006, "parameter cannot be empty")
	}
	if err := common.AddrCalibrator(args.Address); err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	addr := common.StrB58ToAddress(args.Address)
	if err := handler.Wallet.SetDefault(addr); err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	return nil

}

func (handler *WalletHandler) ExportByAddress(args WalletByAddressArgs, resp *string) error {
	if args.Address == "" {
		return xfsgo.NewRPCError(-1006, "parameter cannot be empty")
	}
	if err := common.AddrCalibrator(args.Address); err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	addr := common.StrB58ToAddress(args.Address)
	pk, err := handler.Wallet.Export(addr)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	*resp = "0x" + hex.EncodeToString(pk)
	return nil
}

func (handler *WalletHandler) ImportByPrivateKey(args WalletImportArgs, resp *string) error {
	if args.Key == "" {
		return xfsgo.NewRPCError(-1006, "parameter cannot be empty")
	}
	if len([]byte(args.Key)) < 70 {
		return xfsgo.NewRPCError(-1006, "Key address rule error")
	}
	keyEnc := args.Key

	if keyEnc[0] == '0' && keyEnc[1] == 'x' {
		keyEnc = keyEnc[2:]
	} else {
		return xfsgo.NewRPCError(-1006, "Binary forward backward error")
	}
	keyDer, err := hex.DecodeString(keyEnc)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	addr, err := handler.Wallet.Import(keyDer)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	*resp = addr.B58String()
	return nil
}

func (handler *WalletHandler) SendTransaction(args SendTransactionArgs, resp *string) error {
	var (
		err   error
		stdTx = new(xfsgo.StdTransaction)
	)
	// Judge that the transfer amount cannot be blank
	if args.Value == "" {
		return xfsgo.NewRPCError(-1006, "value not be empty")
	}

	// Get the wallet address of the initiating transaction
	var fromAddr common.Address
	if args.From != "" {
		// from Verify address rules
		if err := common.AddrCalibrator(args.From); err != nil {
			return xfsgo.NewRPCErrorCause(-6001, err)
		}
		fromAddr = common.B58ToAddress([]byte(args.From))
	} else {
		fromAddr = handler.Wallet.GetDefault()
	}

	// Take out the private key according to the wallet address of the initiating transaction
	privateKey, err := handler.Wallet.GetKeyByAddress(fromAddr)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}
	// to Verify address rules
	if err = common.AddrCalibrator(args.To); err != nil {
		return xfsgo.NewRPCErrorCause(-6001, err)
	}
	stdTx.To = common.StrB58ToAddress(args.To)
	if args.GasLimit != "" {
		stdTx.GasLimit = common.ParseString2BigInt(args.GasLimit)
	} else {
		stdTx.GasLimit = common.TxGas
	}
	if args.GasPrice != "" {
		gaspriceBig, ok := new(big.Int).SetString(args.GasPrice, 10)
		if !ok {
			return xfsgo.NewRPCError(-1006, "string to big.Int error")
		}
		stdTx.GasPrice = common.NanoCoin2Atto(gaspriceBig)
	} else {
		stdTx.GasPrice = common.DefaultGasPrice()
	}
	stdTx.Value, err = common.BaseCoin2Atto(args.Value)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}
	if args.Nonce != "" {
		nonceBig, ok := new(big.Int).SetString(args.Nonce, 10)
		if !ok {
			return xfsgo.NewRPCError(-1006, "string to big.Int error")
		}
		stdTx.Nonce = nonceBig.Uint64()
	} else {
		state := handler.TxPendingPool.State()
		stdTx.Nonce = state.GetNonce(fromAddr)
	}
	stdTx.Type = xfsgo.TxType(args.Type)

	tx := xfsgo.NewTransactionByStd(stdTx)
	err = tx.SignWithPrivateKey(privateKey)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}
	err = handler.TxPendingPool.Add(tx)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}

	// Judgment target address cannot be empty
	if args.To == "" {
		return xfsgo.NewRPCError(-1006, "to addr not be empty")
	}

	result := tx.Hash()
	*resp = result.Hex()
	return nil
}

func (handler *WalletHandler) Contract(args SendTransactionArgs, resp *string) error {
	var (
		err   error
		stdTx = new(xfsgo.StdTransaction)
	)

	// Judgment target address cannot be empty
	stdTx.To = common.B58ToAddress(common.Hex2bytes(args.To))

	code := common.Hex2bytes(args.Code)
	stdTx.Data = code[:]

	// Judge that the transfer amount cannot be blank
	if args.Value == "" {
		return xfsgo.NewRPCError(-1006, "value not be empty")
	}

	// Get the wallet address of the initiating transaction
	var fromAddr common.Address
	if args.From != "" {
		// from Verify address rules
		if err := common.AddrCalibrator(args.From); err != nil {
			return xfsgo.NewRPCErrorCause(-6001, err)
		}
		fromAddr = common.B58ToAddress([]byte(args.From))
	} else {
		fromAddr = handler.Wallet.GetDefault()
	}

	// Take out the private key according to the wallet address of the initiating transaction
	privateKey, err := handler.Wallet.GetKeyByAddress(fromAddr)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}
	// to Verify address rules
	// if err = common.AddrCalibrator(args.To); err != nil {
	// 	return xfsgo.NewRPCErrorCause(-6001, err)
	// }
	stdTx.To = common.StrB58ToAddress(args.To)
	if args.GasLimit != "" {
		stdTx.GasLimit = common.ParseString2BigInt(args.GasLimit)
	} else {
		stdTx.GasLimit = common.TxGas
	}
	if args.GasPrice != "" {
		gaspriceBig, ok := new(big.Int).SetString(args.GasPrice, 10)
		if !ok {
			return xfsgo.NewRPCError(-1006, "string to big.Int error")
		}
		stdTx.GasPrice = common.NanoCoin2Atto(gaspriceBig)
	} else {
		stdTx.GasPrice = common.DefaultGasPrice()
	}
	stdTx.Value, err = common.BaseCoin2Atto(args.Value)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}
	if args.Nonce != "" {
		nonceBig, ok := new(big.Int).SetString(args.Nonce, 10)
		if !ok {
			return xfsgo.NewRPCError(-1006, "string to big.Int error")
		}
		stdTx.Nonce = nonceBig.Uint64()
	} else {
		state := handler.TxPendingPool.State()
		stdTx.Nonce = state.GetNonce(fromAddr)
	}
	stdTx.Type = xfsgo.TxType(args.Type)

	tx := xfsgo.NewTransactionByStd(stdTx)
	err = tx.SignWithPrivateKey(privateKey)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}
	err = handler.TxPendingPool.Add(tx)
	if err != nil {
		return xfsgo.NewRPCErrorCause(-1006, err)
	}

	result := tx.Hash()
	*resp = result.Hex()
	return nil
}
