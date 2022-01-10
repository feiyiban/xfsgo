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
	"xfsgo"
	"xfsgo/common"

	"github.com/spf13/cobra"
)

var (
	getTxpoolCommand = &cobra.Command{
		Use:                   "txpool <command> [options]",
		DisableFlagsInUseLine: true,
		Short:                 "transaction pool info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	getGetPendingCommand = &cobra.Command{
		Use:                   "pending [options]",
		DisableFlagsInUseLine: true,
		Short:                 "get transaction pool pending queue",
		RunE:                  GetPending,
	}
	getGetQueueCommand = &cobra.Command{
		Use:   "queue  [options]",
		Short: "get transaction pool queue",
		RunE:  GetQueue,
	}
	getGetTranCommand = &cobra.Command{
		Use:                   "gettx [options] <transaction_hash>",
		DisableFlagsInUseLine: true,
		Short:                 "get transaction pool pending by transaction hash",
		RunE:                  GetTransaction,
	}
	getTxpoolCountCommand = &cobra.Command{
		Use:                   "count [options]",
		DisableFlagsInUseLine: true,
		Short:                 "transaction pool transaction number",
		RunE:                  runTxPoolCount,
	}
	clearTxPoolCommand = &cobra.Command{
		Use:                   "clear [options]",
		DisableFlagsInUseLine: true,
		Short:                 "delete all transactions from the local transaction pool",
		RunE:                  ClearTxPool,
	}
	removeTxCommand = &cobra.Command{
		Use:                   "removetx [options] <transaction_hash>",
		DisableFlagsInUseLine: true,
		Short:                 "deletes the specified transaction hash in the local transaction pool",
		RunE:                  RemoveTx,
	}
)

func RemoveTx(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	var result string
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	req := &removeTxArgs{
		Hash: args[0],
	}
	err = cli.CallMethod(1, "TxPool.RemoveTx", &req, &result)
	if err != nil {
		return err
	}
	return nil
}

func ClearTxPool(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	var result string
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "TxPool.Clear", nil, &result)
	if err != nil {
		return err
	}
	return nil
}

func GetPending(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	var txPending *TransactionsResp
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "TxPool.GetPending", nil, &txPending)
	if err != nil {
		return err
	}
	if txPending == nil {
		fmt.Println(txPending)
		return nil
	}
	bs, err := common.MarshalIndent(txPending)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

func GetQueue(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	var txQueue *TransactionsResp
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "TxPool.GetQueue", nil, &txQueue)
	if err != nil {
		return err
	}
	if txQueue == nil {
		fmt.Println("Not found data")
		return nil
	}
	bs, err := common.MarshalIndent(txQueue)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

func runTxPoolCount(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var txPoolCount int
	err = cli.CallMethod(1, "TxPool.GetPendingSize", nil, &txPoolCount)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(txPoolCount)
	return nil
}

func GetTransaction(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Printf(args[0])
	res := make(map[string]interface{}, 1)
	hash := &getTranByHashArgs{
		Hash: args[0],
	}

	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "TxPool.GetTranByHash", &hash, &res)
	if err != nil {
		return err
	}
	if len(res) < 1 {
		fmt.Println("Not found data")
	}
	bs, err := common.MarshalIndent(res)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))

	return nil
}

func init() {
	rootCmd.AddCommand(getTxpoolCommand)
	getTxpoolCommand.AddCommand(getTxpoolCountCommand)
	getTxpoolCommand.AddCommand(getGetPendingCommand)
	getTxpoolCommand.AddCommand(getGetQueueCommand)
	getTxpoolCommand.AddCommand(getGetTranCommand)
	getTxpoolCommand.AddCommand(clearTxPoolCommand)
	getTxpoolCommand.AddCommand(removeTxCommand)
}
