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
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"strings"
	"xfsgo/common"
	"xfsgo/storage/badger"

	"github.com/sirupsen/logrus"
)

var (
	MainNetGenesisBits = uint32(267386909)
	TestNetGenesisBits = uint32(4278190110)
	GenesisBits        = MainNetGenesisBits
)

// WriteGenesisBlock constructs the genesis block for the blockchain and stores it in the hd.
func WriteGenesisBlock(stateDB, chainDB badger.IStorage, reader io.Reader) (*Block, error) {
	return WriteGenesisBlockN(stateDB, chainDB, reader, false)
}

func WriteGenesisBlockN(stateDB, chainDB badger.IStorage, reader io.Reader, debug bool) (*Block, error) {
	contents, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	// Genesis specifies the header fields, state of a genesis block. It also defines accounts
	var genesis struct {
		Version       uint32 `json:"version"`
		HashPrevBlock string `json:"hash_prev_block"`
		Timestamp     string `json:"timestamp"`
		Coinbase      string `json:"coinbase"`
		Bits          uint32 `json:"bits"`
		Nonce         uint32 `json:"nonce"`
		ExtraNonce    uint64 `json:"extranonce"`
		Accounts      map[string]struct {
			Balance string `json:"balance"`
		} `json:"accounts"`
	}
	if err = json.Unmarshal(contents, &genesis); err != nil {
		return nil, err
	}
	chaindb := newChainDBN(chainDB, debug)
	stateTree := NewStateTree(stateDB, nil)
	//logrus.Debugf("initialize genesis account count: %d", len(genesis.Accounts))
	for addr, a := range genesis.Accounts {
		address := common.B58ToAddress([]byte(addr))
		balance := common.ParseString2BigInt(a.Balance)
		stateTree.AddBalance(address, balance)
		//logrus.Debugf("initialize genesis account: %s, balance: %d", address, balance)
	}
	stateTree.UpdateAll()
	timestamp := common.ParseString2BigInt(genesis.Timestamp)
	var coinbase common.Address
	if genesis.Coinbase != "" {
		coinbase = common.B58ToAddress([]byte(genesis.Coinbase))
	}
	GenesisBits = genesis.Bits
	rootHash := common.Bytes2Hash(stateTree.Root())
	HashPrevBlock := common.Hex2Hash(genesis.HashPrevBlock)
	block := NewBlock(&BlockHeader{
		Nonce:         genesis.Nonce,
		ExtraNonce:    genesis.ExtraNonce,
		HashPrevBlock: HashPrevBlock,
		Timestamp:     timestamp.Uint64(),
		Coinbase:      coinbase,
		GasUsed:       new(big.Int),
		GasLimit:      common.GenesisGasLimit,
		Bits:          genesis.Bits,
		StateRoot:     rootHash,
	}, nil, nil)
	if old := chaindb.GetBlockHeaderByHash(block.HeaderHash()); old != nil {
		logrus.WithField("hash", old.HashHex()).Infof("Genesis Block")
		oldGeneisBlock := &Block{Header: old, Transactions: nil, Receipts: nil}
		return oldGeneisBlock, nil
	}
	logrus.WithField("hash", block.HashHex()).Infof("Write genesis block")
	//logrus.Infof("write genesis block hash: %s", block.HashHex())
	if err = stateTree.Commit(); err != nil {
		return nil, err
	}
	if err = chaindb.WriteBHeaderWithHash(block.Header); err != nil {
		return nil, err
	}
	if err = chaindb.WriteBHeader2Chain(block.Header); err != nil {
		return nil, err
	}
	return block, nil
}

func WriteMainNetGenesisBlock(stateDB, blockDB badger.IStorage) (*Block, error) {
	return WriteMainNetGenesisBlockN(stateDB, blockDB, false)
}

func WriteMainNetGenesisBlockN(stateDB, blockDB badger.IStorage, debug bool) (*Block, error) {
	bits := MainNetGenesisBits
	jsonStr := fmt.Sprintf(`{
	"nonce": 0,
	"bits": %d,
	"coinbase": "haF8HrbHByusg6VcCqdjZqMrKasKNv7KN"
}`, bits)
	return WriteGenesisBlockN(stateDB, blockDB, strings.NewReader(jsonStr), debug)
}

func WriteTestNetGenesisBlock(stateDB, blockDB badger.IStorage) (*Block, error) {
	return WriteTestNetGenesisBlockN(stateDB, blockDB, false)
}

func WriteTestNetGenesisBlockN(stateDB, blockDB badger.IStorage, debug bool) (*Block, error) {
	jsonStr := fmt.Sprintf(`{
	"nonce": 0,
	"bits": %d,
	"coinbase": "bQfi7kVUf2VAUsBk1R9FEzHXdtNtD98bs",
	"accounts": {
		"bQfi7kVUf2VAUsBk1R9FEzHXdtNtD98bs": {"balance": "600000000000000000000000000"}
	}
}`, TestNetGenesisBits)
	return WriteGenesisBlockN(stateDB, blockDB, strings.NewReader(jsonStr), debug)
}

func WriteTestGenesisBlock(testGenesisBits uint32, stateDB, blockDB badger.IStorage) (*Block, error) {
	return WriteTestGenesisBlockN(testGenesisBits, stateDB, blockDB, false)
}
func WriteTestGenesisBlockN(testGenesisBits uint32, stateDB, blockDB badger.IStorage, debug bool) (*Block, error) {
	jsonStr := fmt.Sprintf(`{
	"nonce": 0,
	"bits": %d,
	"coinbase": "1A2QiH4FYc9c4nsNjCMxygg9HKTK9EJWX5",
	"accounts": {
		"1A2QiH4FYc9c4nsNjCMxygg9HKTK9EJWX5": {"balance": "10000000"}
	}
}`, testGenesisBits)
	return WriteGenesisBlockN(stateDB, blockDB, strings.NewReader(jsonStr), debug)
}
