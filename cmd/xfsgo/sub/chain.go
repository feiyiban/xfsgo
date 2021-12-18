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

// var fileType string = ".car"
var (
	chainCommand = &cobra.Command{
		Use:                   "chain <command> [options]",
		DisableFlagsInUseLine: true,
		Short:                 "show chain info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	chainGetHeadCommond = &cobra.Command{
		Use:                   "gethead [options]",
		DisableFlagsInUseLine: true,
		Short:                 "get the latest block information of local chain data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getHead()
		},
	}
	chainGetBlockByNumCommond = &cobra.Command{
		Use:                   "getblockbynum [options] <number>",
		DisableFlagsInUseLine: true,
		Short:                 "query block information of specified height",
		RunE:                  getBlockByNum,
	}
	chainGetBlockByHashCommond = &cobra.Command{
		Use:                   "getblockbyhash [options] <hash>",
		DisableFlagsInUseLine: true,
		Short:                 "query the block information of the specified hash value",
		RunE:                  getBlockByHash,
	}
	chainGetTxsByBlockNumCommond = &cobra.Command{
		Use:                   "gettxsbyblocknum [options] <number>",
		DisableFlagsInUseLine: true,
		Short:                 "query transaction information of specified block height",
		RunE:                  getTxsByBlockNum,
	}
	chainGetTxsByBlockHashCommond = &cobra.Command{
		Use:                   "gettxsbyblockhash [options] <hash>",
		DisableFlagsInUseLine: true,
		Short:                 "query the transaction information of the specified block hash value",
		RunE:                  getTxsByBlockHash,
	}
	chainGetTransactionCommand = &cobra.Command{
		Use:                   "gettxbyhash [options] <transaction_hash>",
		DisableFlagsInUseLine: true,
		Short:                 "query the transaction information of the specified transaction hash value",
		RunE:                  getTransaction,
	}
	chainGetReceiptByHashCommand = &cobra.Command{
		Use:                   "getreceiptbytxhash [options] <transaction_hash> ",
		DisableFlagsInUseLine: true,
		Short:                 "query receipt information of specified transaction hash value",
		RunE:                  getReceiptByTxHash,
	}
	chainSyncStatusCommand = &cobra.Command{
		Use:                   "syncstatus [options]",
		DisableFlagsInUseLine: true,
		Short:                 "Query block synchronization status",
		RunE:                  getSyncStatus,
	}
	chainCallCommand = &cobra.Command{
		Use:                   "contractcall [options] <contract tx hash> <call code>",
		DisableFlagsInUseLine: true,
		Short:                 "Query contract execute result",
		RunE:                  contractcall,
	}
)

// Gets the header of the highest block
func getHead() error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var block *common.BlocksMap
	err = cli.CallMethod(1, "Chain.Head", nil, &block)
	if err != nil {
		return err
	}
	if block == nil {
		return err
	}
	result := block.MapMerge()
	sortIndex := []string{"version", "height", "hash_prev_block", "hash", "timestamp", "state_root", "transactions_root", "receipts_root", "bits", "nonce", "coinbase", "gas_limit", "gas_used"}
	bs, err := common.Marshal(result, sortIndex, true)
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", bs)
	return nil
}

// Block information according to block height
func getBlockByNum(cmd *cobra.Command, args []string) error {

	// Required parameters
	if len(args) < 1 {
		return cmd.Help()
	}

	// Service configuration
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var result *common.BlocksMap
	req := &getBlockByNumArgs{
		Number: args[0],
	}
	err = cli.CallMethod(1, "Chain.GetBlockByNumber", &req, &result)
	if err != nil {
		return err
	}
	if result == nil {
		return err
	}
	// sortIndex := []string{"version", "height", "hash_prev_block", "hash", "timestamp", "state_root", "transactions_root", "receipts_root", "bits", "nonce", "coinbase", "gas_limit", "gas_used"}
	bs, err := common.MarshalIndent(result)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

// Get blocks from hash
func getBlockByHash(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}

	config, err := parseClientConfig(cfgFile)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var block *common.BlocksMap
	req := &getBlockByHashArgs{
		Hash: args[0],
	}
	err = cli.CallMethod(1, "Chain.GetBlockByHash", &req, &block)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if block == nil {
		fmt.Println(block)
		return nil
	}

	sortIndex := []string{"version", "height", "hash_prev_block", "hash", "timestamp", "state_root", "transactions_root", "receipts_root", "bits", "nonce", "coinbase", "gas_limit", "gas_used"}
	result := block.MapMerge()
	bs, err := common.Marshal(result, sortIndex, true)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(bs)
	return nil
}

// Get all transactions according to block height
func getTxsByBlockNum(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}

	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)

	result := make(map[string]interface{})
	req := &getTxsByBlockNumArgs{
		Number: args[0],
	}
	if err = cli.CallMethod(1, "Chain.GetTxsByBlockNum", &req, &result); err != nil {
		return err
	}

	if len(result) == 0 {
		fmt.Println("Not found")
		return nil
	}
	bs, err := common.MarshalIndent(result)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

// Get all transactions based on block hash
func getTxsByBlockHash(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}

	result := make(map[string]interface{})
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	req := getTxsByBlockHashArgs{
		Hash: args[0],
	}

	if err = cli.CallMethod(1, "Chain.GetTxsByBlockHash", &req, &result); err != nil {
		return err
	}

	if len(result) == 0 {
		fmt.Println("Not found")
		return nil
	}

	bs, err := common.MarshalIndent(result)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

// Get receipt information according to hash
func getReceiptByTxHash(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	result := make(map[string]interface{})
	req := &getReceiptByHashArgs{
		Hash: args[0],
	}
	err = cli.CallMethod(1, "Chain.GetReceiptByHash", &req, &result)
	if err != nil {
		return err
	}
	bs, err := common.MarshalIndent(result)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

// Obtain transaction information according to transaction hash
func getTransaction(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	result := make(map[string]interface{})
	req := &getTransactionArgs{
		Hash: args[0],
	}

	if err = cli.CallMethod(1, "Chain.GetTransaction", &req, &result); err != nil {
		return err
	}

	if len(result) == 0 {
		fmt.Println("Not found")
		return err
	}

	bs, err := common.MarshalIndent(result)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

func getSyncStatus(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	result := make(map[string]interface{})
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	if err = cli.CallMethod(1, "Chain.GetSyncStatus", nil, &result); err != nil {
		return nil
	}
	if len(result) == 0 {
		fmt.Println("Not found data")
		return nil
	}

	bs, err := common.MarshalIndent(result)
	if err != nil {
		return err
	}
	fmt.Println(string(bs))
	return nil
}

func contractcall(cmd *cobra.Command, args []string) error {

	if len(args) < 2 {
		return cmd.Help()
	}

	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}

	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var result string
	req := &sendTransactionArgs{
		Hash: args[0],
		Code: args[1],
	}

	if fromAddr != "" {
		req.From = fromAddr
	}
	if gasLimit != "" {
		req.GasLimit = gasLimit
	}
	if gasPrice != "" {
		req.GasPrice = gasPrice
	}
	if nonce != "" {
		req.Nonce = nonce
	}
	err = cli.CallMethod(1, "Wallet.ContractCall", req, &result)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(result)
	return nil
}

func init() {

	rootCmd.AddCommand(chainCommand)
	chainCommand.AddCommand(chainGetBlockByNumCommond)
	chainCommand.AddCommand(chainGetHeadCommond)
	chainCommand.AddCommand(chainGetTransactionCommand)
	chainCommand.AddCommand(chainGetReceiptByHashCommand)
	chainCommand.AddCommand(chainGetBlockByHashCommond)
	chainCommand.AddCommand(chainGetTxsByBlockHashCommond)
	chainCommand.AddCommand(chainGetTxsByBlockNumCommond)
	chainCommand.AddCommand(chainSyncStatusCommand)
	chainCommand.AddCommand(chainCallCommand)
}
