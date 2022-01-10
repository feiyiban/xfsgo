package sub

import (
	"fmt"
	"xfsgo"

	"github.com/spf13/cobra"
)

var (
	netCommand = &cobra.Command{
		Use:                   "net <command> [options]",
		DisableFlagsInUseLine: true,
		Short:                 "Network related operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	getPeersCommand = &cobra.Command{
		Use:                   "peers [options]",
		DisableFlagsInUseLine: true,
		Short:                 "View established peer-to-peer links",
		RunE:                  getPeers,
	}
	addPeerCommand = &cobra.Command{
		Use:                   "addpeer [options] <url>",
		DisableFlagsInUseLine: true,
		Short:                 "Add peer-to-peer link",
		RunE:                  addPeer,
	}
	delPeerCommand = &cobra.Command{
		Use:                   "delpeer <node_id> [options]",
		DisableFlagsInUseLine: true,
		Short:                 "del peer-to-peer link",
		RunE:                  delPeer,
	}
	getNodeIdCommand = &cobra.Command{
		Use:                   "getid [options]",
		DisableFlagsInUseLine: true,
		Short:                 "View the ID of the current node",
		RunE:                  getNodeId,
	}
)

func getPeers(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	var res []string
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "Net.GetPeers", nil, &res)
	if err != nil {
		return err
	}
	if res == nil || len(res) == 0 {
		fmt.Println("Not found peers")
	}
	for _, peer := range res {
		fmt.Println(peer)
	}
	return nil
}

func addPeer(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	var res string
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	req := &AddPeerArgs{
		Url: args[0],
	}
	err = cli.CallMethod(1, "Net.AddPeer", &req, &res)
	if err != nil {
		return err
	}
	return nil
}

func delPeer(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Help()
	}
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	var res string
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	req := &delPeerArgs{
		Id: args[0],
	}
	err = cli.CallMethod(1, "Net.DelPeer", &req, &res)
	if err != nil {
		return err
	}
	return nil
}

func getNodeId(cmd *cobra.Command, args []string) error {
	config, err := parseClientConfig(cfgFile)
	if err != nil {
		return err
	}
	var res string
	cli := xfsgo.NewClient(config.rpcClientApiHost, config.rpcClientApiTimeOut)
	err = cli.CallMethod(1, "Net.GetNodeId", nil, &res)
	if err != nil {
		return err
	}
	fmt.Printf("%v\n", res)
	return nil
}

func init() {
	rootCmd.AddCommand(netCommand)
	netCommand.AddCommand(getPeersCommand)
	netCommand.AddCommand(addPeerCommand)
	netCommand.AddCommand(delPeerCommand)
	netCommand.AddCommand(getNodeIdCommand)
}
