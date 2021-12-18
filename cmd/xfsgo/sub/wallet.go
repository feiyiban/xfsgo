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
	fromAddr      string
	toAddr        string
	gasLimit      string
	gasPrice      string
	nonce         string
	txtype        int
	walletCommand = &cobra.Command{
		Use:                   "wallet <command> [options]",
		DisableFlagsInUseLine: true,
		Short:                 "get wallet info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	walletListCommand = &cobra.Command{
		Use:                   "list [options]",
		DisableFlagsInUseLine: true,
		Short:                 "get wallet address list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getWalletList()
		},
	}
	walletNewCommand = &cobra.Command{
		Use:                   "new [options]",
		DisableFlagsInUseLine: true,
		Short:                 "Create wallet address",
		RunE: func(cmd *cobra.Command, args []string) error {
			return walletNew()
		},
	}
	walletDelCommand = &cobra.Command{
		Use:                   "del [options] <address>",
		DisableFlagsInUseLine: true,
		Short:                 "Delete wallet <address>",
		RunE:                  walletDel,
	}
	walletSetAddrDefCommand = &cobra.Command{
		Use:                   "setdef [options] <address>",
		DisableFlagsInUseLine: true,
		Short:                 "set default wallet <address>",
		RunE:                  setWalletAddrDef,
	}
	walletGetAddrDefCommand = &cobra.Command{
		Use:                   "getdef [options]",
		DisableFlagsInUseLine: true,
		Short:                 "get default wallet address",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getWalletAddrDef()
		},
	}
	walletExportCommand = &cobra.Command{
		Use:                   "export [options] <address>",
		DisableFlagsInUseLine: true,
		Short:                 "export wallet <address>",
		RunE:                  runWalletExport,
	}
	walletImportCommand = &cobra.Command{
		Use:                   "import [options] <key>",
		DisableFlagsInUseLine: true,
		Short:                 "[options] import wallet <key>",
		RunE:                  runWalletImport,
	}
	walletTransferCommand = &cobra.Command{
		Use:                   "transfer [options] <address> <value>",
		DisableFlagsInUseLine: true,
		Short:                 "Send the transaction to the specified destination address",
		RunE:                  sendTransaction,
	}
	walletContractCommand = &cobra.Command{
		Use:                   "contract [options] <opcode> <value>",
		DisableFlagsInUseLine: true,
		Short:                 "contract the transaction to the specified destination address",
		RunE:                  contract,
	}
)

func sendTransaction(cmd *cobra.Command, args []string) error {

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
		To:    args[0],
		Value: args[1],
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
	err = cli.CallMethod(1, "Wallet.SendTransaction", req, &result)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(result)
	return nil
}

func contract(cmd *cobra.Command, args []string) error {

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
		Code:  args[0],
		Value: args[1],
	}

	if fromAddr != "" {
		req.From = fromAddr
	}

	if toAddr != "" {
		req.To = toAddr
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
	err = cli.CallMethod(1, "Wallet.Contract", req, &result)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(result)
	return nil
}

func walletNew() error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var addr *string = nil
	err = cli.CallMethod(1, "Wallet.Create", nil, &addr)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	fmt.Println(*addr)
	return nil
}

func walletDel(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	addr := args[0]
	addrq := &getWalletByAddressArgs{
		Address: addr,
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var r *interface{} = nil
	err = cli.CallMethod(1, "Wallet.Del", addrq, &r)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	fmt.Println("Successfully deleted address")
	return nil
}

func runWalletExport(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	addr := args[0]
	addrq := &getWalletByAddressArgs{
		Address: addr,
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var r *string = nil
	err = cli.CallMethod(1, "Wallet.ExportByAddress", addrq, &r)
	if err != nil {
		return nil
	}
	fmt.Printf("%s\n", *r)
	return nil
}

func runWalletImport(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	addr := args[0]
	importrq := &walletImportArgs{
		Key: addr,
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var r *string = nil
	err = cli.CallMethod(1, "Wallet.ImportByPrivateKey", importrq, &r)
	if err != nil {
		return nil
	}
	fmt.Printf("%s\n", *r)
	return nil
}

func setWalletAddrDef(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	addr := args[0]
	req := &setWalletAddrDefArgs{
		Address: addr,
	}
	var r *string = nil
	err = cli.CallMethod(1, "Wallet.SetDefaultAddress", req, &r)
	if err != nil {
		return nil
	}
	fmt.Printf("Successfully set default address\n")
	return nil
}

func getWalletAddrDef() error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	var defStr *string = nil
	err = cli.CallMethod(1, "Wallet.GetDefaultAddress", nil, &defStr)
	if err != nil {
		return err
	}
	fmt.Println(*defStr)
	return nil
}

func getWalletList() error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	//Get wallet default address
	var defAddr common.Address
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "Wallet.GetDefaultAddress", nil, &defAddr)
	if err != nil {
		return err
	}
	// get height and hash
	block := make(map[string]interface{}, 1)
	err = cli.CallMethod(1, "Chain.Head", nil, &block)
	if err != nil {
		return err
	}
	hash := block["state_root"].(string)
	walletAddress := make([]common.Address, 0)
	err = cli.CallMethod(1, "Wallet.List", nil, &walletAddress)
	if err != nil {
		return err
	}
	var balance string
	fmt.Print("Address                            Balance                       Default")
	fmt.Println()
	for _, w := range walletAddress {

		req := &getAccountArgs{
			RootHash: hash,
			Address:  w.B58String(),
		}
		err = cli.CallMethod(1, "State.GetBalance", &req, &balance)
		if err != nil {
			return err
		}
		result, err := common.Atto2BaseRatCoin(balance)
		if err != nil {
			return xfsgo.NewRPCErrorCause(-32001, err)
		}

		rat, _ := result.Float64()
		fmt.Printf("%-35v", w.B58String())
		fmt.Printf("%-30.9f", rat)

		if w == defAddr {
			fmt.Printf("%-10v", "x")
		}
		fmt.Println()
	}
	return nil
}

func init() {
	walletCommand.AddCommand(walletListCommand)
	walletCommand.AddCommand(walletNewCommand)
	walletCommand.AddCommand(walletDelCommand)
	walletCommand.AddCommand(walletImportCommand)
	walletCommand.AddCommand(walletExportCommand)
	walletCommand.AddCommand(walletGetAddrDefCommand)
	walletCommand.AddCommand(walletTransferCommand)

	mFlags := walletTransferCommand.PersistentFlags()
	mFlags.StringVarP(&fromAddr, "address", "a", "", "Set from address")
	mFlags.StringVarP(&gasPrice, "gasprice", "", "", "Set transaction gas price")
	mFlags.StringVarP(&gasLimit, "gaslimit", "", "", "Set transaction gas limit")
	mFlags.StringVarP(&nonce, "nonce", "", "", "Set transaction nonce")
	mFlags.IntVarP(&txtype, "txtype", "", 0, "transaction type, distinguishing election voting")
	walletCommand.AddCommand(walletContractCommand)
	mContractFlags := walletContractCommand.PersistentFlags()
	mContractFlags.StringVarP(&fromAddr, "fromaddress", "a", "", "Set from address")
	mContractFlags.StringVarP(&toAddr, "toaddress", "t", "", "Set to address")
	mContractFlags.StringVarP(&gasPrice, "gasprice", "", "", "Set transaction gas price")
	mContractFlags.StringVarP(&gasLimit, "gaslimit", "", "", "Set transaction gas limit")
	mContractFlags.StringVarP(&nonce, "nonce", "", "", "Set transaction nonce")
	walletCommand.AddCommand(walletSetAddrDefCommand)
	rootCmd.AddCommand(walletCommand)
}
