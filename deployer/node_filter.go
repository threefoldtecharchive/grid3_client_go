// Package deployer is grid deployer
package deployer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-querystring/query"
	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
)

// NodeFilter struct for options
type NodeFilter struct {
	CRU int `url:"free_cru,omitempty"` // GB
	MRU int `url:"free_mru,omitempty"` // GB
	SRU int `url:"free_sru,omitempty"` // GB
	HRU int `url:"free_hru,omitempty"` // GB

	PublicIPs bool `url:"ipv4,omitempty"`
	Gateway   bool `url:"domain,omitempty"`

	FarmID   string `url:"farm_ids,omitempty"`
	FarmName string `url:"farm_name,omitempty"`
	Country  string `url:"country,omitempty"`
	City     string `url:"city,omitempty"`

	Dedicated bool `url:"dedicated,omitempty"`
	Rentable  bool `url:"rentable,omitempty"`
	Rented    bool `url:"rented,omitempty"`

	AvailableForTwin int `url:"available_for,omitempty"`

	Page   int    `url:"page,omitempty"`
	Status string `url:"status,omitempty"`
}

// FilterNodes filters nodes on a network
func FilterNodes(options NodeFilter, url string) ([]uint32, error) {
	nodes := []uint32{}
	values, _ := query.Values(options)
	query := values.Encode()

	resp, err := http.Get(url + "/nodes?" + query)
	if err != nil {
		return nodes, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nodes, err
	}

	defer resp.Body.Close()

	var nodesData []map[string]interface{}
	err = json.Unmarshal(body, &nodesData)
	if err != nil {
		return nodes, err
	}

	for _, node := range nodesData {
		nodes = append(nodes, uint32(node["nodeId"].(float64)))
	}

	if len(nodes) == 0 {
		return nodes, fmt.Errorf("couldn't find any node with options: %v", query)
	}

	return nodes, nil
}

// FilterNodesWithPublicConfigs filters nodes with public configs
func FilterNodesWithPublicConfigs(sub subi.SubstrateExt, ncPool client.NodeClientGetter, nodes []uint32) ([]uint32, error) {
	filteredNodes := make([]uint32, 0)
	for _, nodeID := range nodes {
		nodeClient, err := ncPool.GetNodeClient(sub, nodeID)
		if err != nil {
			return nodes, err
		}
		_, err = nodeClient.NetworkGetPublicConfig(context.Background())
		if err != nil {
			continue
		}
		filteredNodes = append(filteredNodes, nodeID)
	}

	if len(nodes) == 0 {
		return nodes, fmt.Errorf("couldn't find any node with public configs")
	}

	return filteredNodes, nil
}
