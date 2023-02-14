# **grid3_go:**

grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

- ## **Grid3_go flow:**

  1. the TF plugin will be initialized with identity/network information.
  2. deployer will expose the `Deploy` which the user will call it and give it old/new deployments.
  3. `Deploy` method will do the following
     - internally will calculate changes which
       1. loads old deployments using their ids (node id) from the grid
       2. determine which deployments needs to be created and updated
     - will take the suitable action for each operation to create and update
     - waits on them and report the state
  4. deployer will expose the `Cancel` which the user will call it and give it the contract ID of the deployment he wants to delete.
  5. For applying the changes we have `subi` and `node` package which creates/updates/cancels contracts/deployments on the grid
  6. if a user wants to apply any changes, they should provide their new state, and their current deployment ids.
  7. the deployer should be responsible of reverting the applied changes if some error happens midway.

- ### **How the deployer calculates changes:**

- the deployment manager receives information about old and new deployments, then decides which operations need to be performed.

  1. Old deployments are the ids of the previous deployed stuff and new deployments are the new required state
  2. Incase this is the first time to deploy stuff the oldDeployments map should be empty
  3. Incase the user wants to update those deployments the oldDeployments map should contains the ids from previous deploy request
  4. deployments that need to be created are present in the new deployments, and not in the old deployments.
  5. deployments that need to be updated are present in both old and new deployments, but they must have different hashes.

- ### **Creating a new Deployment:**

  1. deployments must first be signed and validated.
  2. a deployment contract then should be created.
  3. Then, the deployment should be deployed on the node.
  4. If some error happens while trying to deploy on the node, the contract will be canceled to avoid leaking a contract if cancelling contract failed, an error should be reported to the user.
  5. after deployment creation, the function should only return after waiting for 4 minutes on all workloads to be StateOK.

- ### **Updating a deployment:**

  1. a deployment should be updated if and only if hashes differ or a workload name was changed.
  2. if a deployment should be updated, its version should be incremented.
  3. if a workload should be updated, its version should match the deployment version.
  4. if a workload shouldn't be updated, its version stays the same.
  5. the deployment contract should then, be updated.
  6. The deployment on the node should be updated.
  7. after deployment update, the function should only return after waiting on all workloads to be StateOK.

- ### **Deleting a deployment:**

  1. If all deployments on a contract are deleted the contract it self should be canceled as well

- ### **Generating a versionless deployment used by each customized deployer:**

  1. Versionless deployment means to create a deployment object regardless the version, version will be added afterwards depends on if it is new or we need to update it, just deployment builder not affecting the chain at this stage
  2. A user should first generate the appropriate deployment using the `workloads` package.
  3. The `workloads` package then generates the grid.Deployment objects.

- ### **Reverting a deployment:**

  1. before applying any change, the deployer should first retrieve the `currentState` from the nodes `state`.
  2. every contract deletion or creation, should directly be reflected in the `currentState`.
  3. if some error happens while applying some change, the deployer should revert to its old state using the `currentState` as the `oldDeploymentIDs` and the `oldState` as the `newDeployments`.

- ### **Retrieving current state:**

  1. grid3_go users should mainly keep track of contract ids returned from the `Deploy`.
  2. the deployer should use the provided contract ids to retrieve current state from nodes (using `client` package).

  Example:

  - oldDeploymentIDs :1 - newDeployments: 1, 2, 3
  - desired state: 1 update, 2 create, 3 create
  - deployer.deploy(oldDeploymentIDs[1], newDeployments[1,2,3])
  - error happens: 1 updated, 2 created, 3 created
  - currentState: 1, 2, 3

## **grid3_go has the following components:**

- ### **TFPluginClient:**

  - Includes all information about the user (Identity, Mnemonics and Twin ID).
  - Includes all the client that will be used for the grid client.
  - Includes grid proxy client, RMB client, substrate connection, node client.

  - Includes and Supports all the deployers including:
    - VMs, QSFSs, Disks, ZDBs (Deployment Deployer)
    - Name gateways (Name Gateway Deployer)
    - FQDN gateways (FQDN Gateway Deployer)
    - Kubernetes (K8S Deployer)
    - Networks (Network Deployer)

  - Includes a state loader to load all supported zos workloads from the grid state.

- ### **Deployer:**

  - Calculates needed changes between different provided states.
  - Supports deploy, update and cancel operations.

    ```go
    type Deployer interface{
        Deploy(ctx, current [uint32]uint64, new [uint32]gridDeployment, new [uint32]SolutionProvider) (current map[uint32]uint64, error)
        Cancel(ctx, contractID uint64) error
        GetDeployments(ctx, current [uint32]uint64) (current [uint32]gridDeployment, error)
    }
    ```

- ### **Supported Deployers:**

  - Deployment Deployer (VMs, QSFSs, Disks, ZDBs)

    ```go
    type DeploymentDeployer interface{
        GenerateVersionlessDeployments(ctx, workloads.Deployment) (new map[uint32]gridtypes.Deployment, error)
        Validate(ctx, workloads.Deployment) error
        Deploy(ctx, workloads.Deployment) error
        Cancel(ctx, workloads.Deployment) error
        Sync(ctx, workloads.Deployment) error
    }
    ```

  - Name Gateway Deployer (Name gateways)

    ```go
    type NameGatewayDeployer interface{
        GenerateVersionlessDeployments(ctx, workloads.GatewayNameProxy) (new map[uint32]gridtypes.Deployment, error)
        Validate(ctx, workloads.GatewayNameProxy) error
        Deploy(ctx, workloads.GatewayNameProxy) error
        Cancel(ctx, workloads.GatewayNameProxy) error
        Sync(ctx, workloads.GatewayNameProxy) error
    }
    ```

  - FQDN Gateway Deployer (FQDN gateways)

    ```go
    type FQDNGatewayDeployer interface{
        GenerateVersionlessDeployments(ctx, workloads.GatewayFQDNProxy) (new map[uint32]gridtypes.Deployment, error)
        Validate(ctx, workloads.GatewayFQDNProxy) error
        Deploy(ctx, workloads.GatewayFQDNProxy) error
        Cancel(ctx, workloads.GatewayFQDNProxy) error
        Sync(ctx, workloads.GatewayFQDNProxy) error
    }
    ```

  - K8S Deployer (Kubernetes)

    ```go
    type K8sDeployer interface{
        GenerateVersionlessDeployments(ctx, workloads.K8sCluster) (new map[uint32]gridtypes.Deployment, error)
        Validate(ctx, workloads.K8sCluster) error
        Deploy(ctx, workloads.K8sCluster) error
        Cancel(ctx, workloads.K8sCluster) error
        Sync(ctx, workloads.K8sCluster) error
    }
    ```

  - Network Deployer (Networks)

    ```go
    type NetworkDeployer interface{
        GenerateVersionlessDeployments(ctx, workloads.ZNet) (new map[uint32]gridtypes.Deployment, error)
        Validate(ctx, workloads.ZNet) error
        Deploy(ctx, workloads.ZNet) error
        Cancel(ctx, workloads.ZNet) error
        Sync(ctx, workloads.ZNet) error
    }
    ```

- ### **State:**

  - save all current deployments and networks
  - loads any workload from grid

- ### **NodeClient:**

  - Uses grid proxy to get information about nodes, farms, and/or twins.
  - Uses rmb client (from grid proxy) to interact with nodes.

- ### **Subi:**

  - Exposes an interface to interact with the chain.
  - Allows mocking substrate-client for testing.

- ### **Workers:**

  - It is responsible for conversions between grid workloads/types and the grid client workloads/types.
  - It supports the following: deployments, disks, gateways, k8s, networks, publicIP workloads, vms, QSFS, zlog, zdb

### Example

```go
// Create Threefold plugin client
tfPlugin, err := deployer.NewTFPluginClient(mnemonics, "sr25519", network, "", "", true, true)

// Get a suitable node to deploy
filter := NodeFilter{
    CRU:    2,
    SRU:    2,
    MRU:    1,
    Status: "up",
}
nodeIDs, err := FilterNodes(filter, deployer.RMBProxyURLs[tfPluginClient.Network])
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

// Deploy
err = tfPluginClient.NetworkDeployer.Deploy(ctx, &network)

// Load using the state loader
// this loader should load the deployment as json then convert it to a deployment go object with workloads inside it
networkObj, err := tfPluginClient.State.LoadNetworkFromGrid(network.Name)

// Cancel
err = tfPluginClient.NetworkDeployer.Cancel(ctx, &network)
```
