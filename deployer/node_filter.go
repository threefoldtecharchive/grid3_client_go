// Package deployer is grid deployer
package deployer

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	proxy "github.com/threefoldtech/grid_proxy_server/pkg/client"
	proxyTypes "github.com/threefoldtech/grid_proxy_server/pkg/types"
)

// FilterNodes filters nodes using proxy
func FilterNodes(gridClient proxy.Client, options proxyTypes.NodeFilter) ([]proxyTypes.Node, error) {
	nodes, _, err := gridClient.Nodes(options, proxyTypes.Limit{})
	if err != nil {
		return []proxyTypes.Node{}, errors.Wrap(err, "could not fetch nodes from the rmb proxy")
	}

	if len(nodes) == 0 {
		return nodes, fmt.Errorf("could not find any node with options: %+v", options)
	}

	return nodes, nil
}

var (
	trueVal  = true
	statusUp = "up"
)

// GetPublicNode return public node ID
func GetPublicNode(ctx context.Context, gridClient proxy.Client, preferredNodes []uint32) (uint32, error) {
	preferredNodesSet := make(map[int]struct{})
	for _, node := range preferredNodes {
		preferredNodesSet[int(node)] = struct{}{}
	}

	nodes, err := FilterNodes(gridClient, proxyTypes.NodeFilter{
		IPv4:   &trueVal,
		Status: &statusUp,
	})
	if err != nil {
		return 0, err
	}

	// force add preferred nodes
	nodeMap := make(map[int]struct{})
	for _, node := range nodes {
		nodeMap[node.NodeID] = struct{}{}
	}

	for _, node := range preferredNodes {
		if _, ok := nodeMap[int(node)]; ok {
			continue
		}
		nodeInfo, err := gridClient.Node(node)
		if err != nil {
			log.Error().Msgf("failed to get node %d from the grid proxy", node)
			continue
		}
		if nodeInfo.PublicConfig.Ipv4 == "" {
			continue
		}
		if nodeInfo.Status != "up" {
			continue
		}
		nodes = append(nodes, proxyTypes.Node{
			PublicConfig: nodeInfo.PublicConfig,
		})
	}

	lastPreferred := 0
	for i := range nodes {
		if _, ok := preferredNodesSet[nodes[i].NodeID]; ok {
			nodes[i], nodes[lastPreferred] = nodes[lastPreferred], nodes[i]
			lastPreferred++
		}
	}

	for _, node := range nodes {
		log.Printf("found a node with ipv4 public config: %d %s\n", node.NodeID, node.PublicConfig.Ipv4)
		ip, _, err := net.ParseCIDR(node.PublicConfig.Ipv4)
		if err != nil {
			log.Printf("could not parse public ip %s of node %d: %s", node.PublicConfig.Ipv4, node.NodeID, err.Error())
			continue
		}
		if ip.IsPrivate() {
			log.Printf("public ip %s of node %d is private", node.PublicConfig.Ipv4, node.NodeID)
			continue
		}
		return uint32(node.NodeID), nil
	}

	return 0, errors.New("no nodes with public ipv4")
}
