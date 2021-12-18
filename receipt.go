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
	"encoding/json"
	"math/big"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
)

type Receipt struct {
	Version         uint32         `json:"version"`
	ContractAddress common.Address `json:"contractAddress"`
	Status          uint32         `json:"status"`
	TxHash          common.Hash    `json:"tx_hash"`
	GasUsed         *big.Int       `json:"gas_used"`
}

func NewReceipt(txHash common.Hash) *Receipt {
	return &Receipt{
		Version: version0,
		TxHash:  txHash,
	}
}
func (r *Receipt) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Receipt) Decode(data []byte) error {
	return json.Unmarshal(data, r)
}

func (r *Receipt) Hash() common.Hash {
	bs, err := rawencode.Encode(r)
	if err != nil {
		return common.ZeroHash
	}
	return common.Bytes2Hash(ahash.SHA256(bs))
}
