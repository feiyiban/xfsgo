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

package sub

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"xfsgo"
	"xfsgo/backend"
	"xfsgo/common"
	"xfsgo/node"

	"github.com/spf13/viper"
)

const (
	defaultConfigFile        = "./config.yml"
	defaultStorageDir        = ".xfsgo"
	defaultChainDir          = "chain"
	defaultStateDir          = "state"
	defaultKeysDir           = "keys"
	defaultExtraDir          = "extra"
	defaultNodesDir          = "nodes"
	defaultRPCClientAPIHost  = "127.0.0.1:9012"
	defaultNodeRPCListenAddr = "127.0.0.1:9012"
	defaultNodeP2PListenAddr = "0.0.0.0:9011"
	defaultNetworkId         = uint32(1)
	defaultTestNetworkId     = uint32(2)
	defaultProtocolVersion   = uint32(1)
	defaultLoggerLevel       = "INFO"
	defaultCliTimeOut        = "180s"
	defaultNodeSyncFlag      = true
)

var defaultMinGasPrice = common.DefaultGasPrice()

// var defaultMaxGasLimit = common.MinGasLimit

type storageParams struct {
	dataDir  string
	chainDir string
	keysDir  string
	stateDir string
	extraDir string
	nodesDir string
}

type loggerParams struct {
	level string
}

type daemonConfig struct {
	loggerParams  loggerParams
	storageParams storageParams
	nodeConfig    node.Config
	backendParams backend.Params
}

type clientConfig struct {
	rpcClientApiHost    string
	rpcClientApiTimeOut string
}

var defaultNumWorkers = uint32(runtime.NumCPU())

func readFromConfigPath(v *viper.Viper, customFile string) error {
	filename := filepath.Base(defaultConfigFile)
	ext := filepath.Ext(defaultConfigFile)
	configPath := filepath.Dir(defaultConfigFile)
	v.AddConfigPath("$HOME/.xfsgo")
	v.AddConfigPath("/etc/xfsgo")
	v.AddConfigPath(configPath)
	v.SetConfigType(strings.TrimPrefix(ext, "."))
	v.SetConfigName(strings.TrimSuffix(filename, ext))
	v.SetConfigFile(customFile)
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	return nil
}

func parseConfigLoggerParams(v *viper.Viper) loggerParams {
	params := loggerParams{}
	params.level = v.GetString("logger.level")
	if params.level == "" {
		params.level = defaultLoggerLevel
	}
	return params
}

func setupDataDir(params *storageParams, datadir string) {
	if datadir != "" && params.dataDir != datadir {
		np := new(storageParams)
		np.dataDir = datadir
		*params = *np
	}
	if params.chainDir == "" {
		params.chainDir = filepath.Join(
			params.dataDir, defaultChainDir)
	}
	if params.stateDir == "" {
		params.stateDir = filepath.Join(
			params.dataDir, defaultStateDir)
	}
	if params.keysDir == "" {
		params.keysDir = filepath.Join(
			params.dataDir, defaultKeysDir)
	}
	if params.extraDir == "" {
		params.extraDir = filepath.Join(
			params.dataDir, defaultExtraDir)
	}
	if params.nodesDir == "" {
		params.nodesDir = filepath.Join(
			params.dataDir, defaultNodesDir)
	}
}

func parseConfigStorageParams(v *viper.Viper) storageParams {
	params := storageParams{}
	params.dataDir = v.GetString("storage.datadir")
	params.chainDir = v.GetString("storage.chaindir")
	params.stateDir = v.GetString("storage.statedir")
	params.keysDir = v.GetString("storage.keysdir")
	params.extraDir = v.GetString("storage.extradir")
	params.nodesDir = v.GetString("storage.nodesdir")
	if params.dataDir == "" {
		home := os.Getenv("HOME")
		params.dataDir = filepath.Join(
			home, defaultStorageDir)
	}
	setupDataDir(&params, params.dataDir)
	return params
}

func getDefaultNodeSyncFlag() bool {
	return defaultNodeSyncFlag
}

func defaultBootstrapNodes(netid uint32) []string {
	// hardcoded bootstrap nodes
	if netid == 1 {
		// main net boot nodes
		return []string{}
	}
	if netid == 2 {
		// test net boot nodes
		return []string{
			// SG
			"xfsnode://139.180.144.201:9011/?id=f477a4ad00870704fec42af8c056e5f8e799caa1b40c3b50a6e7137a2ece33d03a0a386f19a8fc80f1023455f640c20a903f1c154adecae312c92a072e9d0fc5",
			// JP
			"xfsnode://45.63.126.195:9011/?id=4b2c14c672f8df9fa61d69d7c3f1e2a6334f18a22674d7c66cb1e3adae7a76c4933953970e8e9840734cdb6f9b2a639fd0e79eada381443345e668e45d89f95d",
			// CN
			"xfsnode://119.28.26.67:9011/?id=8c729a1ee6ce899aa42ffd7a03c18e094a4c570f2cfd600ac7310670e0f561e7a1d92980306762c13ea26aba799a42b84626428a778e6a92b5f6b8d36d4099cb",
			// UK
			"xfsnode://78.141.192.47:9011/?id=d4333425281b943a666ce1ae91c3e39d45ea1b7f5f25f62d18e5f9b9cfb1380448d5db9a486ce7242c78795274853712e8360fb02ef89a344f5a5155c3e9f056",
			// US
			"xfsnode://45.32.88.212:9011/?id=8d5e80c4d9d57b7594a465e5b9fbba0c983b40574d8110fc6a50f573e2d78214f8672cd8acf202b14f9b2a6175bb8800eebfd064e243dd81c3571bf56a17171d",
			// AS
			"xfsnode://45.32.243.66:9011/?id=642816f6b1488ca77487a7d9ced3001be3c862efef6f7fc2be0e64cf13f842f3ff936643f5d1b29efd2048870d6609757393f514aa0d0db0004de0630760ecb6",
		}
	}
	return make([]string, 0)
}
func parseConfigNodeParams(v *viper.Viper, netid uint32) node.Config {
	config := node.Config{
		RPCConfig: new(xfsgo.RPCConfig),
	}
	config.RPCConfig.ListenAddr = v.GetString("rpcserver.listen")
	config.P2PListenAddress = v.GetString("p2pnode.listen")
	config.P2PBootstraps = v.GetStringSlice("p2pnode.bootstrap")
	config.P2PStaticNodes = v.GetStringSlice("p2pnode.static")
	config.ProtocolVersion = uint8(v.GetUint64("protocol.version"))
	config.NodeSyncFlag = v.GetBool("p2pnode.syncflag")

	if config.RPCConfig.ListenAddr == "" {
		config.RPCConfig.ListenAddr = defaultNodeRPCListenAddr
	}
	if config.P2PListenAddress == "" {
		config.P2PListenAddress = defaultNodeP2PListenAddr
	}
	if config.P2PBootstraps == nil || len(config.P2PBootstraps) == 0 {
		config.P2PBootstraps = defaultBootstrapNodes(netid)
	}

	if config.NodeSyncFlag || getDefaultNodeSyncFlag() {
		config.NodeSyncFlag = true
	}

	return config
}

func parseConfigBackendParams(v *viper.Viper) backend.Params {
	config := backend.Params{}
	mCoinbase := v.GetString("miner.coinbase")
	if mCoinbase != "" {
		config.Coinbase = common.StrB58ToAddress(mCoinbase)
	}
	minGasPriceStr := v.GetString("miner.gasprice")
	config.ProtocolVersion = v.GetUint32("protocol.version")
	config.NetworkID = v.GetUint32("protocol.networkid")
	config.Numworkers = v.GetUint32("miner.numworkers")
	var minGasPrice *big.Int
	var ok bool
	if minGasPrice, ok = new(big.Int).SetString(minGasPriceStr, 10); !ok || minGasPrice.Cmp(defaultMinGasPrice) < 0 {
		minGasPrice = defaultMinGasPrice
	}
	config.MinGasPrice = minGasPrice
	if config.Numworkers == uint32(0) {
		config.Numworkers = defaultNumWorkers
	}

	if config.ProtocolVersion == 0 {
		config.ProtocolVersion = defaultProtocolVersion
	}
	if config.NetworkID == 0 {
		config.NetworkID = defaultNetworkId
	}
	config.GenesisFile = v.GetString("protocol.genesisfile")
	return config
}

func parseDaemonConfig(configFilePath string) (daemonConfig, error) {
	config := viper.New()
	if err := readFromConfigPath(config, configFilePath); err != nil && configFilePath != "" {
		return daemonConfig{}, err
	}
	mStorageParams := parseConfigStorageParams(config)
	mBackendParams := parseConfigBackendParams(config)
	mLoggerParams := parseConfigLoggerParams(config)
	nodeParams := parseConfigNodeParams(config, mBackendParams.NetworkID)
	nodeParams.NodeDBPath = mStorageParams.nodesDir
	return daemonConfig{
		loggerParams:  mLoggerParams,
		storageParams: mStorageParams,
		nodeConfig:    nodeParams,
		backendParams: mBackendParams,
	}, nil
}

func parseClientConfig(configFilePath string) (clientConfig, error) {
	config := viper.New()
	if err := readFromConfigPath(config, configFilePath); err != nil && configFilePath != "" {
		return clientConfig{}, err
	}
	mRpcClientApiHost := config.GetString("rpclient.apihost")
	if rpchost == "" {
		if mRpcClientApiHost == "" {
			mRpcClientApiHost = fmt.Sprintf("http://%s", defaultRPCClientAPIHost)
		}
	} else {
		mRpcClientApiHost = fmt.Sprintf("http://%s", rpchost)
	}
	mRpcClientApiTimeOut := config.GetString("rpclient.timeout")
	if mRpcClientApiTimeOut == "" {
		mRpcClientApiTimeOut = defaultCliTimeOut
	}
	timeDur, err := time.ParseDuration(mRpcClientApiTimeOut)
	if err != nil {
		return clientConfig{}, err
	}
	times := timeDur.Seconds()
	if times < 1 && times > 3*60 {
		return clientConfig{}, fmt.Errorf("time overflow")
	}
	return clientConfig{
		rpcClientApiHost:    mRpcClientApiHost,
		rpcClientApiTimeOut: mRpcClientApiTimeOut,
	}, nil
}
