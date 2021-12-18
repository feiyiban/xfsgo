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

package backend

import (
	"errors"
	"log"
	"math/big"
	"os"
	"xfsgo"
	"xfsgo/common"
	"xfsgo/consensus"
	"xfsgo/consensus/dpos"
	"xfsgo/miner"
	"xfsgo/node"
	"xfsgo/p2p"
	"xfsgo/params"
	"xfsgo/storage/badger"

	"github.com/sirupsen/logrus"
)

var (
	ErrInitialGenesis    = errors.New("initial genesis block fail")
	ErrMainNetDisabled   = errors.New("main net disabled")
	ErrWriteGenesisBlock = errors.New("write genesis block err")
)

// Backend represents the backend server of the xfs and implements the xfs full node service.
type Backend struct {
	config     *Config
	blockchain *xfsgo.BlockChain
	p2pServer  p2p.Server
	wallet     *xfsgo.Wallet
	miner      *miner.Miner
	eventBus   *xfsgo.EventBus
	txPool     *xfsgo.TxPool
	syncMgr    *syncMgr
	engine     consensus.Engine
}

type Params struct {
	NetworkID       uint32
	GenesisFile     string
	Coinbase        common.Address
	Numworkers      uint32
	ProtocolVersion uint32
	Debug           bool
	MinGasPrice     *big.Int
}

// Config contains the configuration options of the Backend.
type Config struct {
	*Params
	NodeSyncFlag bool
	ChainDB      *badger.Storage
	KeysDB       *badger.Storage
	StateDB      *badger.Storage
	ExtraDB      *badger.Storage
}
type chainSyncProtocol struct {
	syncMgr *syncMgr
}

func (c *chainSyncProtocol) Run(p p2p.Peer) error {
	return c.syncMgr.onNewPeer(p)
}

// NewBackend constructs and returns a Backend instance by a note in network and config.
// This method is for daemon whick should be started firstly when xfs blockchain runs.
//
func NewBackend(stack *node.Node, config *Config) (*Backend, error) {
	var err error = nil
	back := &Backend{
		config:    config,
		p2pServer: stack.P2PServer(),
		engine:    dpos.New(&params.DposConfig{}, config.ChainDB),
	}
	back.eventBus = xfsgo.NewEventBus()
	if config.NetworkID == uint32(1) {
		if xfsgo.VersionMajor() != 1 {
			return nil, ErrMainNetDisabled
		}
		if _, err = xfsgo.WriteMainNetGenesisBlockN(
			back.config.StateDB, back.config.ChainDB, config.Params.Debug); err != nil {
			return nil, ErrWriteGenesisBlock
		}
	} else if config.NetworkID == uint32(2) {
		if _, err = xfsgo.WriteTestNetGenesisBlockN(
			back.config.StateDB, back.config.ChainDB, config.Params.Debug); err != nil {
			return nil, err
		}
	} else if len(config.GenesisFile) > 0 {
		var fr *os.File
		if fr, err = os.Open(config.GenesisFile); err != nil {
			return nil, ErrWriteGenesisBlock
		}
		if _, err = xfsgo.WriteGenesisBlockN(
			back.config.StateDB, back.config.ChainDB, fr, config.Params.Debug); err != nil {
			return nil, ErrWriteGenesisBlock
		}
		_ = fr.Close()
	} else {
		return nil, ErrInitialGenesis
	}
	if back.blockchain, err = xfsgo.NewBlockChainN(
		back.config.StateDB, back.config.ChainDB,
		back.config.ExtraDB, back.eventBus,
		config.Debug); err != nil {
		return nil, err
	}
	back.wallet = xfsgo.NewWallet(back.config.KeysDB)
	back.txPool = xfsgo.NewTxPool(
		back.blockchain.CurrentStateTree,
		back.blockchain.LatestGasLimit,
		back.config.MinGasPrice, back.eventBus)
	coinbase := config.Coinbase
	addrdef := back.wallet.GetDefault()
	if !coinbase.Equals(common.Address{}) || addrdef.Equals(common.Address{}) {
		coinbase, err = back.wallet.AddByRandom()
		if err != nil {
			return nil, err
		}
		if err = back.wallet.SetDefault(coinbase); err != nil {
			return nil, err
		}
	}
	//constructs Miner instance.
	minerconfig := &miner.Config{
		Coinbase:   back.wallet.GetDefault(),
		Numworkers: config.Numworkers,
	}

	back.miner = miner.NewMiner(minerconfig,
		back.config.StateDB, back.blockchain,
		back.eventBus, back.txPool,
		config.MinGasPrice, common.TxPoolGasLimit, back.engine, config.ChainDB)

	logrus.Debugf("Initial miner: coinbase=%s, gasPrice=%s, gasLimit=%s",
		minerconfig.Coinbase.B58String(), config.MinGasPrice, common.TxPoolGasLimit)
	//Node resgisters apis of baclend on the node  for RPC service.
	if err = stack.RegisterBackend(
		back.config.StateDB,
		back.blockchain,
		back.miner,
		back.wallet,
		back.txPool); err != nil {
		return nil, err
	}
	back.syncMgr = newSyncMgr(
		back.config.ProtocolVersion, back.config.NetworkID,
		back.blockchain, back.eventBus, back.txPool, back.config.NodeSyncFlag)
	back.p2pServer.Bind(&chainSyncProtocol{
		syncMgr: back.syncMgr,
	})
	return back, nil
}

func (b *Backend) Start() error {
	b.syncMgr.Start()
	return nil
}

func (b *Backend) BlockChain() *xfsgo.BlockChain {
	return b.blockchain
}
func (b *Backend) EventBus() *xfsgo.EventBus {
	return b.eventBus
}

func (b *Backend) StateDB() *badger.Storage {
	return b.config.StateDB
}

func (b *Backend) close() {
	if err := b.config.ChainDB.Close(); err != nil {
		log.Fatalf("Blocks Storage close errors: %s", err)
	}
	if err := b.config.KeysDB.Close(); err != nil {
		log.Fatalf("Blocks Storage close errors: %s", err)
	}
}
