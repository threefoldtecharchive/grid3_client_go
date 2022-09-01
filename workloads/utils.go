package workloads

import (
	"context"
	"fmt"
	"time"

	client "github.com/threefoldtech/grid3-go/node"
	"github.com/threefoldtech/grid3-go/subi"
)

func isIn(l []uint16, i uint16) bool {
	for _, x := range l {
		if i == x {
			return true
		}
	}
	return false
}

func isInByte(l []byte, i byte) bool {
	for _, x := range l {
		if i == x {
			return true
		}
	}
	return false
}

func isInUint32(l []uint32, i uint32) bool {
	for _, x := range l {
		if i == x {
			return true
		}
	}
	return false
}

func isInStr(l []string, i string) bool {
	for _, x := range l {
		if i == x {
			return true
		}
	}
	return false
}

func isNodesUp(ctx context.Context, sub subi.SubstrateExt, nodes []uint32, nc client.NodeClientCollection) error {
	for _, node := range nodes {
		cl, err := nc.GetNodeClient(sub, node)
		if err != nil {
			return fmt.Errorf("couldn't get node %d client: %w", node, err)
		}
		if err := isNodeUp(ctx, cl); err != nil {
			return fmt.Errorf("couldn't reach node %d: %w", node, err)
		}
	}

	return nil
}

func isNodeUp(ctx context.Context, nc *client.NodeClient) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := nc.NetworkListInterfaces(ctx)
	if err != nil {
		return err
	}

	return nil
}
