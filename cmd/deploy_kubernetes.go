// Package cmd for parsing command line arguments
package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	command "github.com/threefoldtech/grid3-go/internal/cmd"
	"github.com/threefoldtech/grid3-go/workloads"
)

var k8sFlist = "https://hub.grid.tf/tf-official-apps/threefoldtech-k3s-latest.flist"

// deployKubernetesCmd represents the deploy kubernetes command
var deployKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "Deploy a kubernetes cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		sshFile, err := cmd.Flags().GetString("ssh")
		if err != nil {
			return err
		}
		sshKey, err := os.ReadFile(sshFile)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		masterNode, err := cmd.Flags().GetUint32("master-node")
		if err != nil {
			return err
		}
		masterCPU, err := cmd.Flags().GetInt("master-cpu")
		if err != nil {
			return err
		}
		masterRAM, err := cmd.Flags().GetInt("master-ram")
		if err != nil {
			return err
		}
		masterDisk, err := cmd.Flags().GetInt("master-disk")
		if err != nil {
			return err
		}
		ipv4, err := cmd.Flags().GetBool("ipv4")
		if err != nil {
			return err
		}
		ipv6, err := cmd.Flags().GetBool("ipv6")
		if err != nil {
			return err
		}
		ygg, err := cmd.Flags().GetBool("ygg")
		if err != nil {
			return err
		}
		master := workloads.K8sNode{
			Name:      name,
			Node:      masterNode,
			CPU:       masterCPU,
			Memory:    masterRAM * 1024,
			DiskSize:  masterDisk,
			Flist:     k8sFlist,
			PublicIP:  ipv4,
			PublicIP6: ipv6,
			Planetary: ygg,
		}

		workerNumber, err := cmd.Flags().GetInt("workers-number")
		if err != nil {
			return err
		}

		workersNode, err := cmd.Flags().GetUint32("workers-node")
		if err != nil {
			return err
		}
		if workersNode == 0 {
			workersNode = masterNode
		}
		workersCPU, err := cmd.Flags().GetInt("workers-cpu")
		if err != nil {
			return err
		}
		workersRAM, err := cmd.Flags().GetInt("workers-ram")
		if err != nil {
			return err
		}
		workersDisk, err := cmd.Flags().GetInt("workers-disk")
		if err != nil {
			return err
		}
		var workers []workloads.K8sNode
		for i := 0; i < workerNumber; i++ {
			workerName := fmt.Sprintf("worker%d", i)
			worker := workloads.K8sNode{
				Name:     workerName,
				Node:     workersNode,
				Flist:    k8sFlist,
				CPU:      workersCPU,
				Memory:   workersRAM * 1024,
				DiskSize: workersDisk,
			}
			workers = append(workers, worker)
		}

		cluster, err := command.DeployKubernetesCluster(master, workers, string(sshKey))
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		if ipv4 {
			log.Info().Msgf("master ipv4: %s", cluster.Master.ComputedIP)
		}
		if ipv6 {
			log.Info().Msgf("master ipv6: %s", cluster.Master.ComputedIP6)
		}
		if ygg {
			log.Info().Msgf("master yggdrasil ip: %s", cluster.Master.YggIP)
		}
		return nil
	},
}

func init() {
	deployCmd.AddCommand(deployKubernetesCmd)

	deployKubernetesCmd.Flags().StringP("name", "n", "", "name of the kuberentes cluster")
	err := deployKubernetesCmd.MarkFlagRequired("name")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployKubernetesCmd.Flags().String("ssh", "", "path to public ssh key")
	err = deployKubernetesCmd.MarkFlagRequired("ssh")
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	deployKubernetesCmd.Flags().Int("master-cpu", 1, "master number of cpu units")
	deployKubernetesCmd.Flags().Int("master-ram", 1, "master memory size in gb")
	deployKubernetesCmd.Flags().Int("master-disk", 2, "master disk size in gb")
	deployKubernetesCmd.Flags().Uint32("master-node", 0, "master node id")

	deployKubernetesCmd.Flags().Int("workers-number", 0, "number of workers")
	deployKubernetesCmd.Flags().Int("workers-cpu", 1, "workers number of cpu units")
	deployKubernetesCmd.Flags().Int("workers-ram", 1, "workers memory size in gb")
	deployKubernetesCmd.Flags().Int("workers-disk", 2, "workers disk size in gb")
	deployKubernetesCmd.Flags().Uint32("workers-node", 0, "workers node id")

	deployKubernetesCmd.Flags().Bool("ipv4", false, "assign public ipv4 for master")
	deployKubernetesCmd.Flags().Bool("ipv6", false, "assign public ipv6 for master")
	deployKubernetesCmd.Flags().Bool("ygg", true, "assign yggdrasil ip for master")
}
