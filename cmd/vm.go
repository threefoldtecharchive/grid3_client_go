// Package cmd for parsing command line arguments
package cmd

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/threefoldtech/grid3-go/deployer"
	"github.com/threefoldtech/grid3-go/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
)

var ubuntuFlist = "https://hub.grid.tf/tf-official-apps/threefoldtech-ubuntu-22.04.flist"
var ubuntuFlistEntrypoint = "/sbin/zinit init"

// vmCmd represents the vm command
var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Deploy a vm",
	RunE: func(cmd *cobra.Command, args []string) error {
		vm := workloads.VM{}
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		vm.Name = name
		sshFile, err := cmd.Flags().GetString("ssh")
		if err != nil {
			return err
		}
		sshKey, err := os.ReadFile(sshFile)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		vm.EnvVars = map[string]string{"SSH_KEY": string(sshKey)}
		cpu, err := cmd.Flags().GetInt("cpu")
		if err != nil {
			return err
		}
		vm.CPU = cpu
		ram, err := cmd.Flags().GetInt("ram")
		if err != nil {
			return err
		}
		vm.Memory = ram * 1024
		rootfs, err := cmd.Flags().GetInt("rootfs")
		if err != nil {
			return err
		}
		disk, err := cmd.Flags().GetInt("disk")
		if err != nil {
			return err
		}
		mounts := []workloads.Disk{}
		if disk != 0 {
			diskName := fmt.Sprintf("%sdisk", name)
			mount := workloads.Disk{Name: diskName, SizeGB: disk * 1024}
			mounts = append(mounts, mount)
			vm.Mounts = []workloads.Mount{{DiskName: diskName, MountPoint: "/data"}}
		}
		vm.RootfsSize = rootfs * 1024
		flist, err := cmd.Flags().GetString("flist")
		if err != nil {
			return err
		}
		vm.Flist = flist
		entrypoint, err := cmd.Flags().GetString("entrypoint")
		if err != nil {
			return err
		}
		vm.Entrypoint = entrypoint
		ipv4, err := cmd.Flags().GetBool("ipv4")
		if err != nil {
			return err
		}
		vm.PublicIP = ipv4
		ipv6, err := cmd.Flags().GetBool("ipv6")
		if err != nil {
			return err
		}
		vm.PublicIP6 = ipv6
		ygg, err := cmd.Flags().GetBool("ygg")
		if err != nil {
			return err
		}
		vm.Planetary = ygg
		// TODO: get mnemonics and network from login command
		mnemonics := os.Getenv("MNEMONICS")
		gridNetwork := os.Getenv("NETWORK")
		tfclient, err := deployer.NewTFPluginClient(mnemonics, "sr25519", gridNetwork, "", "", "", true, false)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		networkName := fmt.Sprintf("%snetwork", name)
		// TODO: node not hardcoded
		network := workloads.ZNet{
			Name:  networkName,
			Nodes: []uint32{14},
			IPRange: gridtypes.NewIPNet(net.IPNet{
				IP:   net.IPv4(10, 20, 0, 0),
				Mask: net.CIDRMask(16, 32),
			}),
			SolutionType: name,
		}

		vm.NetworkName = networkName
		dl := workloads.NewDeployment(vm.Name, 14, name, nil, networkName, mounts, nil, []workloads.VM{vm}, nil)

		log.Info().Msg("deploying network")
		err = tfclient.NetworkDeployer.Deploy(context.Background(), &network)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msg("deploying vm")
		err = tfclient.DeploymentDeployer.Deploy(context.Background(), &dl)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		resVM, err := tfclient.State.LoadVMFromGrid(14, vm.Name, dl.Name)
		if err != nil {
			log.Fatal().Err(err).Send()
		}

		if ipv4 {
			log.Info().Msgf("vm ipv4: %s\n", resVM.ComputedIP)
		}
		if ipv6 {
			log.Info().Msgf("vm ipv6: %s\n", resVM.ComputedIP6)
		}
		if ygg {
			log.Info().Msgf("vm yggdrasil ip: %s\n", resVM.YggIP)
		}
		return nil
	},
}

func init() {
	deployCmd.AddCommand(vmCmd)

	vmCmd.Flags().StringP("name", "n", "", "name of ther virutal machine")
	err := vmCmd.MarkFlagRequired("name")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	vmCmd.Flags().String("ssh", "", "path to public ssh key")
	// should it be required?
	err = vmCmd.MarkFlagRequired("ssh")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	vmCmd.Flags().Int("cpu", 1, "number of cpu units")
	vmCmd.Flags().Int("ram", 1, "memory size in gb")
	vmCmd.Flags().Int("rootfs", 2, "root filesystem size in gb")
	vmCmd.Flags().Int("disk", 0, "disk size in gb mounted on /data")
	vmCmd.Flags().String("flist", ubuntuFlist, "flist for vm")
	vmCmd.Flags().String("entrypoint", ubuntuFlistEntrypoint, "entrypoint for vm")
	// to ensure entrypoint is provided for custom flist
	vmCmd.MarkFlagsRequiredTogether("flist", "entrypoint")

	vmCmd.Flags().Bool("ipv4", false, "assign public ipv4 for vm")
	vmCmd.Flags().Bool("ipv6", false, "assign public ipv6 for vm")
	vmCmd.Flags().Bool("ygg", true, "assign yggdrasil ip for vm")
}
