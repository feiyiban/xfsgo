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

package node

import (
	"crypto/ecdsa"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"xfsgo"
	"xfsgo/api"
	"xfsgo/common"
	"xfsgo/common/rawencode"
	"xfsgo/crypto"
	"xfsgo/miner"
	"xfsgo/p2p"
	"xfsgo/p2p/discover"
	"xfsgo/p2p/nat"
	"xfsgo/storage/badger"

	"github.com/sirupsen/logrus"
)

// Node is a container on which services can be registered.
type Node struct {
	// *Opts
	config    *Config
	p2pServer p2p.Server
	rpcServer *xfsgo.RPCServer
}

type Config struct {
	P2PListenAddress string
	ProtocolVersion  uint8
	P2PBootstraps    []string
	P2PStaticNodes   []string
	NodeDBPath       string
	RPCConfig        *xfsgo.RPCConfig
	NodeSyncFlag     bool
}

const datadirPrivateKey = "NODEKEY"

// New creates a new P2P node, ready for protocol registration.
func New(config *Config) (*Node, error) {
	bootstraps := make([]*discover.Node, 0)
	for _, nodeUri := range config.P2PBootstraps {
		node, err := discover.ParseNode(nodeUri)
		if err != nil {
			logrus.Warnf("Parse node uri err: %s", err)
		}
		bootstraps = append(bootstraps, node)
	}
	staticNodes := make([]*discover.Node, 0)
	for _, nodeUri := range config.P2PStaticNodes {
		node, err := discover.ParseNode(nodeUri)
		if err != nil {
			logrus.Warnf("Parse node uri err: %s", err)
		}
		staticNodes = append(staticNodes, node)
	}
	enc := new(rawencode.StdEncoder)
	//logrus.Infof("logger level: %s", logrus.GetLevel())
	p2pServer := p2p.NewServer(p2p.Config{
		Encoder:        enc,
		Nat:            nat.Any(),
		ListenAddr:     config.P2PListenAddress,
		Key:            nodeKeyByPath(config.NodeDBPath),
		BootstrapNodes: bootstraps,
		StaticNodes:    staticNodes,
		Discover:       true,
		MaxPeers:       10,
		NodeDBPath:     config.NodeDBPath,
		Logger:         logrus.StandardLogger(),
	})
	n := &Node{
		config:    config,
		p2pServer: p2pServer,
	}
	n.rpcServer = xfsgo.NewRPCServer(config.RPCConfig)
	return n, nil
}

// Start starts p2p networking and RPC services runs in a goroutine.
// Node can only be started once.
func (n *Node) Start() error {
	if err := n.p2pServer.Start(); err != nil {
		return err
	}
	go func() {
		if err := n.rpcServer.Start(); err != nil {
			logrus.Errorln(err)
		}
	}()
	return nil
}

//RegisterBackend registers built-in APIs.
func (n *Node) RegisterBackend(
	stateDb *badger.Storage,
	bc *xfsgo.BlockChain,
	miner *miner.Miner,
	wallet *xfsgo.Wallet,
	txPool *xfsgo.TxPool) error {
	chainApiHandler := &api.ChainAPIHandler{
		BlockChain:    bc,
		TxPendingPool: txPool,
	}
	minerApiHandler := &api.MinerAPIHandler{
		Miner: miner,
	}

	walletApiHandler := &api.WalletHandler{
		Wallet:        wallet,
		BlockChain:    bc,
		TxPendingPool: txPool,
	}

	txPoolHandler := &api.TxPoolHandler{
		TxPool: txPool,
	}
	stateHandler := &api.StateAPIHandler{
		StateDb:    stateDb,
		BlockChain: bc,
	}
	netAPIHandler := &api.NetAPIHandler{
		NetServer: n.P2PServer(),
	}

	if err := n.rpcServer.RegisterName("Chain", chainApiHandler); err != nil {
		log.Fatalf("RPC service register error: %s", err)
		return err
	}

	if err := n.rpcServer.RegisterName("Wallet", walletApiHandler); err != nil {
		log.Fatalf("RPC service register error: %s", err)
		return err
	}
	if err := n.rpcServer.RegisterName("Miner", minerApiHandler); err != nil {
		log.Fatalf("RPC service register error: %s", err)
		return err
	}
	if err := n.rpcServer.RegisterName("TxPool", txPoolHandler); err != nil {
		log.Fatalf("RPC service register error: %s", err)
		return err
	}
	if err := n.rpcServer.RegisterName("State", stateHandler); err != nil {
		log.Fatalf("RPC service register error: %s", err)
		return err
	}
	if err := n.rpcServer.RegisterName("Net", netAPIHandler); err != nil {
		log.Fatalf("RPC service register error: %s", err)
		return err
	}
	return nil
}

func (n *Node) P2PServer() p2p.Server {
	return n.p2pServer
}

func nodeKeyByPath(pathname string) *ecdsa.PrivateKey {
	if pathname == "" {
		k, err := crypto.GenPrvKey()
		if err != nil {
			logrus.Errorf("Gen random private key err: %s", k)
		}
		return k
	}
	keyfile := filepath.Join(pathname, datadirPrivateKey)
	if file, err := os.OpenFile(keyfile, os.O_RDONLY, 0644); err == nil {
		defer common.Safeclose(file.Close)
		if der, err := ioutil.ReadAll(file); err == nil {
			_, key, err := crypto.DecodePrivateKey(der)
			if err == nil {
				return key
			}
		}
		return nil
	}
	k, err := crypto.GenPrvKey()
	if err != nil {
		return nil
	}
	der := crypto.EncodePrivateKey(0, k)
	if err = os.MkdirAll(pathname, 0700); err != nil {
		logrus.Errorf("Failed to persist node key: %v", err)
		return k
	}
	if err = ioutil.WriteFile(keyfile, der, 0644); err != nil {
		logrus.Errorf("Failed to write node key: %v", err)
		return k
	}
	return k
}
