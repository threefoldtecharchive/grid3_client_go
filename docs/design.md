# **grid3_go:**

grid3_go is a go client created to interact with threefold grid. It should manage CRUD operations for deployments on the grid.

- ## **Grid3_go flow:**

  1. the deployment manager will be initialized with identity/network information.
  2. deployment manager will expose the `Deploy` which the user will call it and give it old/new deployments
  3. `Deploy` method will do the following
     - internally call `calculateChanges` method which
       1. loads old deployments using their ids (node id) from the grid
       2. determine which deployments needs to be created, updated and deleted
     - will take the suitable action for each operation to create, update and delete
     - waits on them and report the state
  4. For applying the changes we have `subi` and `node` package which creates contracts/deployments on the grid
  5. if a user wants to apply any changes, they should provide their new state, and their current deployment ids.
  6. the deployment manager should be responsible of reverting the applied changes if some error happens midway.

- ### **How the deployment manager calculates changes:**

- the deployment manager receives information about old and new deployments, then decides which operations need to be performed.

  1. Old deployments are the ids of the previous deployed stuff and new deployments are the new required state
  2. Incase this is the first time to deploy stuff the oldDeployments map should be empty
  3. Incase the user wants to update those deployments the oldDeployments map should contains the ids from previous deploy request
  4. deployments that need to be created are present in the new deployments, and not in the old deployments.
  5. deployments that need to be deleted are present in the old deployments, and not in the new deployments.
  6. deployments that need to be updated are present in both old and new deployments, but they must have different hashes.

- ### **Creating a new Deployment:**

  1. deployments must first be signed and validated.
  2. a deployment contract then should be created.
  3. Then, the deployment should be deployed on the node.
  4. If some error happens while trying to deploy on the node, the contract will be canceled to avoid leaking a contract if cancelling contract failed, an error should be reported to the user.
  5. after deployment creation, the function should only return after waiting for 4 minutes on all workloads to be StateOK.

- ### **Deleting a deployment:**

  1. If deleting a single deployment this should be canceled, keeping the contract if there are other deployments on it
  2. If all deployments on a contract are deleted the contract it self should be canceled as well
  3. Unneeded public ips should be removed

- ### **Updating a deployment:**

  1. a deployment should be updated if and only if hashes differ or a workload name was changed.
  2. if a deployment should be updated, its version should be incremented.
  3. if a workload should be updated, its version should match the deployment version.
  4. if a workload shouldn't be updated, its version stays the same.
  5. the deployment contract should then, be updated.
  6. The deployment on the node should be updated.
  7. after deployment update, the function should only return after waiting on all workloads to be StateOK.

- ### **Generating a versionless deployment:**

  1. Versionless deployment means to create a deployment object regardless the version, version will be added afterwards depends on if it is new or we need to update it, just deployment builder not affecting the chain at this stage
  2. A user should first generate the appropriate deployment builder using the `builder` package.
  3. The `builder` package then generates the grid.Deployment objects.

- ### **Reverting a deployment:**

  1. before applying any change, the deployment manager should first retrieve the current state from the nodes `oldState`.
  2. at first, `currentState` of deployment ids is provided by the user (as `oldDeploymentIDs`).
  3. every contract deletion or creation, should directly be reflected in the `currentState`.
  4. if some error happens while applying some change, the deployment manager should revert to its old state using the `currentState` as the `oldDeploymentIDs` and the `oldState` as the `newDeployments`.

- ### **Retrieving current state:**

  1. grid3_go users should mainly keep track of contract ids returned from the `Deploy` i.e in terraform this should be save to terraform state to be retrieved on subsequent update calls
  2. the deployment manager should use the provided contract ids to retrieve current state from nodes (using `client` package).

  Example:

  - oldDeploymentIDs :1, 2, 3 - newDeployments: 3, 4, 5
  - desired state: 3 update, 4 create, 5 create
  - deploymentManager.deploy(oldDeploymentIDs[1,2,3], newDeployments[3,4,5])
  - error happens: 1 deleted, 2 not affected, 3 updated, 4 created, 5 not created
  - currentState: 2, 3, 4
  - deploymentManager.deploy(currentState[2,3,4], oldDeployments[1,2,3])

## **grid3_go has the following components:**

- ### **GridClient:**

  - Handles interaction with the chain (Create, Update, Delete).
  - Handles interaction with the nodes. (Create, Update, Delete).
  - Be stateless.

    Example:

```go
type GridClient interface{
    Create(deployments []grid.Deployment) (uint64, error)
    Update(deployments []grid.Deployment) error
    Delete(deploymentIDs []uint64) error
    Wait() error
}
```

- ### **DeploymentManager:**

  - Calculates needed changes between different provided states.
  - Uses the GridClient to apply needed changes.
  - Retrieves current state of specified deployments.

```go
type DeploymentManager interface{
    CalculateChanges(current NodeDeploymentIDMap, new NodeDeploymentMap) ([]create, []update, []delete, error)
    ApplyChanges(current NodeDeploymentIDMap, new NodeDeploymentMap) (current NodeDeploymentIDMap,error)
    GetCurrentDeployments(current NodeDeploymentIDMap) (NodeDeploymentMap, error)
}
```

- ### **NodeClient:**

  - Uses gridproxy to get information about nodes, farms, and/or twins.
  - Uses rmb client (from grid proxy) to interact with nodes.

- ### **Subi:**

  - Exposes an interface to interact with the chain.
  - Allows mocking substrate-client for testing.

- ### **Builder:**

  - builder package is mainly responsible of:

  1. Expose the needed user friendly builder structs.
  2. Generate needed grid deployments from builders to be used by the deployer.

example

```go
manager = Manager.New(identity, net, Mnemonics ....)
// Deploy method takes (oldDeployments, newDeployments)
//oldDeployments as nodeID:deploymentID map
//newDeployments which is the desired state as {nodeID: deploymentObject}
currentDeployments = manager.Deploy({}, {nodeID: deploymentObject})
// incase we want to update those created deployments afterwards
currentDeployments = manager.deploy({nodeID: deploymentId}, {nodeID:deploymentObj})
//using the deployer loader
DeploymentObj = deployer.loadDeployment(deploymentId)
// this loader should load the deployment as json then convert it to deployment go object with workloads inside it

```
