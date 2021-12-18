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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"xfsgo/backend"
	"xfsgo/log"
	"xfsgo/node"
	"xfsgo/storage/badger"

	"github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var (
	rpcaddr          string
	p2paddr          string
	datadir          string
	bootstrap        string
	testnet          bool
	debug            bool
	disableBootstrap bool
	netid            int
	daemonCmd        = &cobra.Command{
		Use:                   "daemon [options]",
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		Short:                 "Start a xfsgo daemon process",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
	}
)

func safeclose(fn func() error) {
	if err := fn(); err != nil {
		panic(err)
	}
}

func resetConfig(config *daemonConfig) {
	if datadir != "" {
		setupDataDir(&config.storageParams, datadir)
		config.nodeConfig.NodeDBPath = config.storageParams.nodesDir
	}
	if rpcaddr != "" {
		config.nodeConfig.RPCConfig.ListenAddr = rpcaddr
	}
	if p2paddr != "" {
		config.nodeConfig.P2PListenAddress = p2paddr
	}
	if netid != 0 {
		config.backendParams.NetworkID = uint32(netid)
	}
	if testnet {
		config.backendParams.NetworkID = defaultTestNetworkId
		if config.nodeConfig.P2PBootstraps == nil || len(config.nodeConfig.P2PBootstraps) == 0 {
			config.nodeConfig.P2PBootstraps = defaultBootstrapNodes(defaultTestNetworkId)
		}
	}
	if disableBootstrap {
		config.nodeConfig.P2PBootstraps = make([]string, 0)
	} else if bootstrap != "" {
		config.nodeConfig.P2PBootstraps = strings.Split(bootstrap, ",")
	}
}
func runDaemon() error {
	var (
		err   error            = nil
		stack *node.Node       = nil
		back  *backend.Backend = nil
	)

	config, err := parseDaemonConfig(cfgFile) // default config
	if err != nil {
		return err
	}
	resetConfig(&config) // input config
	loglevel, err := logrus.ParseLevel(config.loggerParams.level)
	if err != nil {
		return err
	}

	logrus.SetFormatter(&log.Formatter{})
	logrus.SetLevel(loglevel)
	nodeConf := &config.nodeConfig
	nodeConf.RPCConfig.Logger = logrus.StandardLogger()
	if stack, err = node.New(nodeConf); err != nil {
		return err
	}
	chainDb, err := badger.New(config.storageParams.chainDir)
	if err != nil {
		return err
	}
	keysDb, err := badger.New(config.storageParams.keysDir)
	if err != nil {
		return err
	}
	stateDB, err := badger.New(config.storageParams.stateDir)
	if err != nil {
		return err
	}
	extraDB, err := badger.New(config.storageParams.extraDir)
	if err != nil {
		return err
	}
	defer func() {
		safeclose(chainDb.Close)
		safeclose(keysDb.Close)
		safeclose(stateDB.Close)
		safeclose(extraDB.Close)
	}()
	backparams := &config.backendParams
	backparams.Debug = debug
	if backparams.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debugf("Set debug mode")
	}
	if back, err = backend.NewBackend(stack, &backend.Config{
		Params:       backparams,
		NodeSyncFlag: config.nodeConfig.NodeSyncFlag,
		ChainDB:      chainDb,
		KeysDB:       keysDb,
		StateDB:      stateDB,
		ExtraDB:      extraDB,
	}); err != nil {
		return err
	}
	if err = backend.StartNodeAndBackend(stack, back); err != nil {
		return err
	}
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
out:
	select {
	case s := <-c:
		switch s {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			break out
		}
	}
	return nil
}

func init() {
	mFlags := daemonCmd.PersistentFlags()
	mFlags.StringVarP(&rpcaddr, "rpcaddr", "r", "", "Set JSON-RPC Service listen address")
	mFlags.StringVarP(&p2paddr, "p2paddr", "p", "", "Set P2P-Node listen address")
	mFlags.StringVarP(&datadir, "datadir", "d", "", "Set Data directory")
	mFlags.StringVarP(&bootstrap, "bootstrap", "", "", "Specify boot node")
	mFlags.BoolVarP(&testnet, "testnet", "t", false, "Enable test network")
	mFlags.BoolVarP(&disableBootstrap, "dbootstrap", "", false, "Disable Bootstrap")
	mFlags.BoolVarP(&debug, "debug", "", false, "Enable debug")
	mFlags.IntVarP(&netid, "netid", "n", 0, "Explicitly set network id")
	rootCmd.AddCommand(daemonCmd)
}
