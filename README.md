# Grid3_client_go

[![Codacy Badge](https://app.codacy.com/project/badge/Grade/cd6e18aac6be404ab89ec160b4b36671)](https://www.codacy.com/gh/threefoldtech/grid3-go/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=threefoldtech/grid3-go&amp;utm_campaign=Badge_Grade) <a href='https://github.com/jpoles1/gopherbadger' target='_blank'>![gopherbadger-tag-do-not-edit](https://img.shields.io/badge/Go%20Coverage-56%25-brightgreen.svg?longCache=true&style=flat)</a> [![Dependabot](https://badgen.net/badge/Dependabot/enabled/green?icon=dependabot)](https://dependabot.com/)

Grid3-client-go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

## Requirements

[Go](https://golang.org/doc/install) >= 1.19

## Examples

This is a simple example to deploy a VM with a network.

```go
import (
    "github.com/threefoldtech/grid3-go/deployer"
    "github.com/threefoldtech/grid3-go/workloads"
)

// Create Threefold plugin client
tfPluginClient, err := deployer.NewTFPluginClient(mnemonics, "sr25519", network, "", "", true, true)

// Get a free node to deploy
filter := deployer.NodeFilter{
    CRU:    2,
    SRU:    2,
    MRU:    1,
    Status: "up",
}
nodeIDs, err := deployer.FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
nodeID := nodeIDs[0]

// Create a new network to deploy
network := workloads.ZNet{
    Name:        "newNetwork",
    Description: "A network to deploy",
    Nodes:       []uint32{nodeID},
    IPRange: gridtypes.NewIPNet(net.IPNet{
      IP:   net.IPv4(10, 1, 0, 0),
      Mask: net.CIDRMask(16, 32),
    }),
    AddWGAccess: true,
}

// Create a new VM to deploy
vm := workloads.VM{
    Name:       "vm",
    Flist:      "https://hub.grid.tf/tf-official-apps/base:latest.flist",
    CPU:        2,
    PublicIP:   true,
    Planetary:  true,
    Memory:     1024,
    RootfsSize: 20 * 1024,
    Entrypoint: "/sbin/zinit init",
    EnvVars: map[string]string{
        "SSH_KEY": publicKey,
    },
    IP:          "10.20.2.5",
    NetworkName: network.Name,
}

// Deploy the network first
err := tfPluginClient.NetworkDeployer.Deploy(context.Background(), &network)

// Load the network using the state loader
// this loader should load the deployment as json then convert it to a deployment go object with workloads inside it
networkObj, err := tfPluginClient.State.LoadNetworkFromGrid(network.Name)

// Deploy the VM deployment
dl := workloads.NewDeployment("vm", nodeID, "", nil, network.Name, nil, nil, []workloads.VM{vm}, nil)
err = tfPluginClient.DeploymentDeployer.Deploy(ctx, &dl)

// Load the vm using the state loader
vmObj, err := tfPluginClient.State.LoadVMFromGrid(nodeID, vm.Name)

// Cancel the VM deployment
err = tfPluginClient.NetworkDeployer.Cancel(ctx, &dl)

// Cancel the network
err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
```

Refer to [integration examples](https://github.com/threefoldtech/grid3-go/tree/development/integration_tests) directory for more examples.

## CLI

Threefold CLI to manage deployments on Threefold Grid.

### Usage

First [build](#build) tf-grid binaries and move the binary to any `$PATH` directories, for example:

```bash
mv bin/tf-grid /usr/local/bin
```

Login using your [mnemonics](https://threefoldtech.github.io/info_grid/dashboard/portal/dashboard_portal_polkadot_create_account.html) and specify which grid network (mainnet/testnet) to deploy on by running:

```bash
tf-grid login
```

Deploy a VM:

```bash
tf-grid deploy vm --name examplevm --ssh ~/.ssh/id_rsa.pub --cpu 2 --memory 4 --disk 10
```

You should get an output like this if VM name is unique:

```bash
12:06PM INF deploying network
12:06PM INF deploying vm
12:07PM INF vm yggdrasil ip: 300:e9c4:9048:57cf:7da2:ac99:99db:8821
```

Get deployed VM:

```bash
tf-grid get vm examplevm
```

You should see an output like this:

```bash
12:08PM INF vm yggdrasil ip: 300:e9c4:9048:57cf:7da2:ac99:99db:8821
```

Cancel deployed VM:

```bash
tf-grid cancel examplevm
```

You should see an output like this:

```bash
12:10PM INF canceling contracts for project examplevm
12:10PM INF examplevm canceled
```

For detailed description of tf-grid commands check out:

- [vm](docs/cli/vm.md)
- [gateway-fqdn](docs/cli/gateway-fqdn.md)
- [gateway-name](docs/cli/gateway-name.md)
- [kubernetes](docs/cli/kubernetes.md)

### Configuration

tf-grid saves user configuration in `.tfgridconfig` under default configuration directory for your system see: [UserConfigDir()](https://pkg.go.dev/os#UserConfigDir)

### Build

Clone the repo and run the following command inside the repo directory:

```bash
make build
```

## Run tests

To run the tests, export MNEMONICS and NETWORK

```bash
export MNEMONICS="<mnemonics words>"
export NETWORK="<network>" # dev, qa or test
```

Run the following command

### running unit tests

```bash
make test
```

### running integration tests

```bash
make integration
```
